package deployment

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"mc/config"
)

// providerGatewayBindHost reads the deployment's configured proxy bind.
// Tests replace the seam; production shares the one BindHost authority the
// resident consumes (no second read of the config file).
var providerGatewayBindHost = config.BindHost

// The provider gateway rule opens the host INPUT path from the deployment's
// own docker bridge subnet to the address the provider proxy is explicitly
// bound on. The proxy authenticates every request with a per-launch token
// before it routes anything, so the rule reaches the listener, not the
// providers behind it. The rule is only ever written for a bind the operator
// chose; loopback binds need no rule and are refused one.
//
// This file is the pure logic: parsing, canonicalization, rule text. The
// program executions live in provider_gateway_exec.go behind the seams
// declared here, which tests replace wholesale.

// providerGatewayTag marks every rule this surface creates, so revoke
// deletes exactly its own rules and never an operator-managed one.
const providerGatewayTag = "shz-provider-gateway"

// Execution seams (implemented in provider_gateway_exec.go; tests replace
// the variables wholesale).
var (
	ipRouteOutput     = gatewayIPRoute
	ipBridgeLinks     = gatewayIPLinkBridges
	ufwStateOutput    = gatewayUfwState
	ufwNumberedOutput = gatewayUfwNumbered
	ufwAllowEdit      = gatewayUfwAllow
	ufwDeleteRule     = gatewayUfwDelete
)

// ProviderGatewayStatus is the tri-state answer for one deployment: the rule
// is not needed (loopback bind), present, or absent. Refused carries the
// exact manual command instead of a silent partial action.
type ProviderGatewayStatus struct {
	State        string   `json:"state"` // not-applicable | present | absent | refused
	Bind         string   `json:"bind"`
	Network      string   `json:"network,omitempty"`
	Interface    string   `json:"interface,omitempty"`
	Port         int      `json:"port,omitempty"`
	Detail       string   `json:"detail,omitempty"`
	SuggestedCmd []string `json:"suggested_command,omitempty"`
}

// canonicalPair carries the two values that may ever reach a rule, both
// re-rendered from parses: a canonical IP literal and a canonical CIDR.
type canonicalPair struct {
	Bind    string
	Network string
	Iface   string
}

func canonicalize(bind, network, iface string) (canonicalPair, error) {
	ip := net.ParseIP(bind)
	if ip == nil {
		return canonicalPair{}, fmt.Errorf("the provider proxy bind %q is not an IP literal", bind)
	}
	_, block, err := net.ParseCIDR(network)
	if err != nil {
		return canonicalPair{}, fmt.Errorf("the derived network %q is not a CIDR", network)
	}
	if !block.Contains(ip) {
		return canonicalPair{}, fmt.Errorf("the bind %s is outside the derived network %s", ip.String(), block.String())
	}
	return canonicalPair{Bind: ip.String(), Network: block.String(), Iface: iface}, nil
}

// ruleSpec builds the scoped ufw argument vector from canonical values and
// an strconv-rendered port. The proxy listens on an ephemeral port per
// start, so the default rule spans TCP to the bind address — the bind
// address itself is the exposure surface the operator opted into, and by
// deployment design only the proxy listens on the bridge IP. A caller that
// runs the proxy on a stable port passes it and the rule narrows to that
// single port. ufw spells the narrowing `to <address> port <n>` before the
// protocol; the 218 acceptance arm is what pinned that grammar — the earlier
// trailing form was rejected as improper syntax by real ufw.
func ruleSpec(p canonicalPair, port int) []string {
	args := []string{"allow", "from", p.Network, "to", p.Bind}
	if port > 0 {
		args = append(args, "port", strconv.Itoa(port))
	}
	args = append(args, "proto", "tcp")
	return append(args, "comment", providerGatewayTag)
}

// manualHint is the exact non-executed path for hosts without an active ufw.
func manualHint(p canonicalPair, port int) []string {
	args := []string{"iptables", "-I", "INPUT", "1", "-s", p.Network, "-d", p.Bind, "-p", "tcp"}
	if port > 0 {
		args = append(args, "--dport", strconv.Itoa(port))
	}
	return append(args, "-m", "comment", "--comment", providerGatewayTag, "-j", "ACCEPT")
}

// bindOf resolves the configured bind and the loopback fact. An invalid or
// absent configuration fails the way BindHost does: absence is legal,
// corruption is not.
func bindOf(home string) (string, bool, error) {
	bind, err := providerGatewayBindHost(home)
	if err != nil {
		return "", false, err
	}
	ip := net.ParseIP(bind)
	if ip == nil {
		return bind, false, fmt.Errorf("the provider proxy bind %q is not an IP literal", bind)
	}
	return bind, ip.IsLoopback(), nil
}

// bridgeNetwork finds the subnet the rule may name: a directly-connected
// kernel route (proto kernel, scope link) on an interface the KERNEL reports
// as type bridge, whose network contains the bind. Bridge class comes from
// `ip link show type bridge`, never from a name guess — the first real-host
// run proved names defeat prefixes (the deployment's own bridge is
// `shz-br0`, and `br-*` naming is docker's convention, not a law). The
// kernel type plus on-link scope is what makes the source safe: only
// L2-adjacent residents of that bridge can match it. No such route, no
// rule — the source is never guessed.
func bridgeNetwork(routeOutput, bridgeLinksOutput, bind string) (canonicalPair, error) {
	ip := net.ParseIP(bind)
	if ip == nil {
		return canonicalPair{}, fmt.Errorf("the provider proxy bind %q is not an IP literal", bind)
	}
	bridges := map[string]bool{}
	for _, line := range strings.Split(bridgeLinksOutput, "\n") {
		// `ip -o link show type bridge` lines lead with "N: name: <...>"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(strings.TrimSuffix(fields[1], ":"), "@if0")
		bridges[name] = true
	}
	for _, line := range strings.Split(routeOutput, "\n") {
		if !strings.Contains(line, "proto kernel") || !strings.Contains(line, "scope link") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		_, block, err := net.ParseCIDR(fields[0])
		if err != nil {
			continue
		}
		iface := ""
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				iface = fields[i+1]
			}
		}
		if !bridges[iface] {
			continue
		}
		if block.Contains(ip) {
			return canonicalize(bind, block.String(), iface)
		}
	}
	return canonicalPair{}, fmt.Errorf("the bind %s is not inside a directly-attached kernel-classified bridge network; refusing to guess a source", bind)
}

// numberedLineMatchesRule decides whether one ufw status line IS the rule the
// caller asked about. A portless request matches any tagged rule for the bind
// (the wide shape the tool itself adds by default). A port-scoped request
// matches only a line carrying that port — real ufw echoes a port rule as
// "<bind> <port>/tcp" — so a wide rule never satisfies a scoped status or
// idempotence check, and vice versa the scoped add stays honest.
func numberedLineMatchesRule(line, bind string, port int) bool {
	if !strings.Contains(line, providerGatewayTag) || !strings.Contains(line, bind) {
		return false
	}
	if port <= 0 {
		return true
	}
	return strings.Contains(line, bind+" "+strconv.Itoa(port)+"/tcp")
}

// matchedTaggedRules lists the rule indices carrying this tool's tag for one
// bind. port<=0 matches every tagged rule for the bind (revoke sweeps them all;
// a portless status asks "is any gateway rule present"); port>0 matches only a
// rule scoped to that exact port (idempotence and a scoped status).
func matchedTaggedRules(numberedStatus, bind string, port int) []int {
	var numbers []int
	for _, line := range strings.Split(numberedStatus, "\n") {
		if !numberedLineMatchesRule(line, bind, port) {
			continue
		}
		// ufw numbers rules as "[ 1]" — bracketed, spaced, digits. Strip
		// every bracket first, then take the first remaining token: that
		// is the index whatever the spacing.
		open := strings.NewReplacer("[", " ", "]", " ").Replace(line)
		tokens := strings.Fields(open)
		if len(tokens) == 0 {
			continue
		}
		n, err := strconv.Atoi(tokens[0])
		if err == nil && n > 0 {
			numbers = append(numbers, n)
		}
	}
	return numbers
}

func ufwActiveNow() (bool, error) {
	out, err := ufwStateOutput()
	if err != nil {
		return false, nil // missing or unreadable ufw is the manual path, not an error
	}
	return strings.Contains(strings.ToLower(out), "status: active"), nil
}

// resolveGatewayPair separates the two failure classes the surface must
// treat differently: a corrupt or unreadable bind configuration (a real
// error — the deployment state itself cannot be trusted) versus a bind whose
// source network cannot be derived (a refusal — the request is understood,
// the rule would need a guess, so nothing is done and the reason is shown).
func resolveGatewayPair(home string) (bind string, loopback bool, pair canonicalPair, derived bool, err error) {
	bind, loopback, err = bindOf(home)
	if err != nil {
		return bind, false, canonicalPair{}, false, err
	}
	if loopback {
		return bind, true, canonicalPair{}, false, nil
	}
	routes, err := ipRouteOutput()
	if err != nil {
		return bind, false, canonicalPair{}, false, fmt.Errorf("read host routes: %w", err)
	}
	bridges, err := ipBridgeLinks()
	if err != nil {
		return bind, false, canonicalPair{}, false, fmt.Errorf("read bridge interfaces: %w", err)
	}
	pair, perr := bridgeNetwork(routes, bridges, bind)
	if perr != nil {
		return bind, false, canonicalPair{}, false, nil
	}
	return bind, false, pair, true, nil
}

// ProviderGatewayAllow creates the scoped rule for the deployment's explicit
// bind. It refuses loopback binds, unbacked source derivations, and hosts
// without an active ufw — in every refusal it prints the exact manual
// command instead of touching anything. It is idempotent by rule match.
func ProviderGatewayAllow(home string, port int) (ProviderGatewayStatus, error) {
	bind, loopback, pair, derived, err := resolveGatewayPair(home)
	if err != nil {
		return ProviderGatewayStatus{}, err
	}
	if loopback {
		return ProviderGatewayStatus{State: "refused", Bind: bind,
			Detail: "the provider proxy is bound to loopback; containers reach it without a host rule, so adding one would open what nothing needs"}, nil
	}
	if !derived {
		return ProviderGatewayStatus{State: "refused", Bind: bind,
			Detail: "the bind is not inside any locally-attached bridge network; the rule source would have to be guessed, so nothing was changed"}, nil
	}
	active, err := ufwActiveNow()
	if err != nil {
		return ProviderGatewayStatus{}, err
	}
	if !active {
		return ProviderGatewayStatus{State: "refused", Bind: bind, Network: pair.Network,
			Interface:    pair.Iface,
			Detail:       "no active ufw was found; this tool will not guess the host firewall backend, apply the exact command below yourself",
			SuggestedCmd: manualHint(pair, port)}, nil
	}
	numbered, err := ufwNumberedOutput()
	if err != nil {
		return ProviderGatewayStatus{}, fmt.Errorf("read ufw rules: %w", err)
	}
	if len(matchedTaggedRules(numbered, pair.Bind, port)) > 0 {
		return ProviderGatewayStatus{State: "present", Bind: bind, Network: pair.Network,
			Interface: pair.Iface, Port: port,
			Detail: "the scoped rule already exists; nothing changed"}, nil
	}
	if err := ufwAllowEdit(ruleSpec(pair, port)); err != nil {
		return ProviderGatewayStatus{}, fmt.Errorf("add the provider gateway rule: %w", err)
	}
	return ProviderGatewayStatus{State: "present", Bind: bind, Network: pair.Network,
		Interface: pair.Iface, Port: port,
		Detail: "the scoped rule was added; revoke deletes only rules carrying this tool's comment tag"}, nil
}

// ProviderGatewayRevoke removes exactly the tagged rules naming one bind.
// Untagged rules are never touched; an absent rule is a no-op.
func ProviderGatewayRevoke(home string) (ProviderGatewayStatus, error) {
	bind, loopback, pair, derived, err := resolveGatewayPair(home)
	if err != nil {
		return ProviderGatewayStatus{}, err
	}
	if loopback {
		return ProviderGatewayStatus{State: "not-applicable", Bind: bind,
			Detail: "a loopback bind never needed a rule"}, nil
	}
	if !derived {
		// without the derivation the tagged-rule search still runs by
		// bind text alone: revoking one's own tagged rules never needs a
		// source guess, and untagged rules stay untouched by design.
		pair = canonicalPair{Bind: bind}
	}
	active, err := ufwActiveNow()
	if err != nil {
		return ProviderGatewayStatus{}, err
	}
	if !active {
		return ProviderGatewayStatus{State: "refused", Bind: bind,
			Detail: "no active ufw; remove manual rules by their comment tag " + providerGatewayTag}, nil
	}
	numbered, err := ufwNumberedOutput()
	if err != nil {
		return ProviderGatewayStatus{}, fmt.Errorf("read ufw rules: %w", err)
	}
	numbers := matchedTaggedRules(numbered, pair.Bind, 0)
	if len(numbers) == 0 {
		return ProviderGatewayStatus{State: "absent", Bind: bind,
			Detail: "no tagged rule for this bind exists; nothing was removed"}, nil
	}
	// delete highest-numbered first so lower indices stay valid mid-sweep
	for i := len(numbers) - 1; i >= 0; i-- {
		if err := ufwDeleteRule(numbers[i]); err != nil {
			return ProviderGatewayStatus{}, fmt.Errorf("delete tagged rule %d: %w", numbers[i], err)
		}
	}
	return ProviderGatewayStatus{State: "absent", Bind: bind,
		Detail: fmt.Sprintf("removed %d tagged rule(s); no untagged rule was touched", len(numbers))}, nil
}

// ProviderGatewayStatusOf answers without changing anything, and carries the
// exact rule shape so operators and scripts can see what this surface would
// manage.
func ProviderGatewayStatusOf(home string, port int) (ProviderGatewayStatus, error) {
	bind, loopback, pair, derived, err := resolveGatewayPair(home)
	if err != nil {
		return ProviderGatewayStatus{}, err
	}
	if loopback {
		return ProviderGatewayStatus{State: "not-applicable", Bind: bind,
			Detail: "the provider proxy is bound to loopback; Linux gateway reach is not configured and the default topology needs no rule"}, nil
	}
	if !derived {
		return ProviderGatewayStatus{State: "absent", Bind: bind,
			Detail: "the bind is not inside a locally-attached bridge network; this surface would refuse to add a rule here"}, nil
	}
	active, err := ufwActiveNow()
	if err != nil {
		return ProviderGatewayStatus{}, err
	}
	if !active {
		return ProviderGatewayStatus{State: "absent", Bind: bind, Network: pair.Network,
			Interface:    pair.Iface,
			Detail:       "no active ufw on this host; the proxy is reachable from the bridge unless something else filters it — this tool neither assumes nor edits that",
			SuggestedCmd: manualHint(pair, port)}, nil
	}
	numbered, err := ufwNumberedOutput()
	if err != nil {
		return ProviderGatewayStatus{}, fmt.Errorf("read ufw rules: %w", err)
	}
	state := "absent"
	if len(matchedTaggedRules(numbered, pair.Bind, port)) > 0 {
		state = "present"
	}
	return ProviderGatewayStatus{State: state, Bind: bind, Network: pair.Network,
		Interface: pair.Iface, Port: port,
		SuggestedCmd: append([]string{"ufw"}, ruleSpec(pair, port)...)}, nil
}
