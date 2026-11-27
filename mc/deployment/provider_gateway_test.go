package deployment

import (
	"fmt"
	"strings"
	"testing"
)

// gatewayFixture replaces the five execution seams and the bind read with
// scripted values, recording every call. The real programs never run here:
// these tests are the pure decision face of the provider gateway surface.
type gatewayFixture struct {
	bind        string
	bindErr     error
	routes      string
	bridgeLinks string
	ufwState    string
	numbered    string
	allowRecs   [][]string
	delRecs     []int
	failAllow   bool
}

// defaultBridgeLinks mirrors `ip -o link show type bridge` output for the
// networks bridgeRoutes declares, including the real-host shape that broke
// the first name-heuristic implementation: a bridge named shz-br0.
const defaultBridgeLinks = `2: docker0: <BROADCAST,MULTICAST> mtu 1500 mode DEFAULT group default
33: br-9f2c11: <BROADCAST,MULTICAST> mtu 1500 mode DEFAULT group default
45: shz-br0: <NO-CARRIER,BROADCAST,MULTICAST> mtu 1500 mode DEFAULT group default`

func (f *gatewayFixture) install(t *testing.T) {
	t.Helper()
	bindSave := providerGatewayBindHost
	routeSave := ipRouteOutput
	bridgeSave := ipBridgeLinks
	stateSave := ufwStateOutput
	numSave := ufwNumberedOutput
	allowSave := ufwAllowEdit
	deleteSave := ufwDeleteRule
	t.Cleanup(func() {
		providerGatewayBindHost = bindSave
		ipRouteOutput = routeSave
		ipBridgeLinks = bridgeSave
		ufwStateOutput = stateSave
		ufwNumberedOutput = numSave
		ufwAllowEdit = allowSave
		ufwDeleteRule = deleteSave
	})
	providerGatewayBindHost = func(string) (string, error) { return f.bind, f.bindErr }
	ipRouteOutput = func() (string, error) { return f.routes, nil }
	bridgeLinks := f.bridgeLinks
	if bridgeLinks == "" {
		bridgeLinks = defaultBridgeLinks
	}
	ipBridgeLinks = func() (string, error) { return bridgeLinks, nil }
	ufwStateOutput = func() (string, error) {
		if f.ufwState == "" {
			return "", fmt.Errorf("ufw: command not found")
		}
		return f.ufwState, nil
	}
	ufwNumberedOutput = func() (string, error) { return f.numbered, nil }
	ufwAllowEdit = func(spec []string) error {
		if f.failAllow {
			return fmt.Errorf("ufw exploded")
		}
		f.allowRecs = append(f.allowRecs, spec)
		return nil
	}
	ufwDeleteRule = func(index int) error {
		f.delRecs = append(f.delRecs, index)
		return nil
	}
}

const bridgeRoutes = `default via 10.0.0.1 dev enp0s3 proto dhcp
10.0.0.0/24 dev enp0s3 proto kernel scope link src 10.0.0.7
172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1 linkdown
10.199.0.0/24 dev br-9f2c11 proto kernel scope link src 10.199.0.1 linkdown
`

// TestGatewayKernelClassifiesBridges is the real-host regression: the
// deployment bridge is named shz-br0 — no name prefix rule covers every
// honest shape, so membership is read from the kernel's interface type.
func TestGatewayKernelClassifiesBridges(t *testing.T) {
	f := &gatewayFixture{bind: "10.199.0.1", routes: `10.199.0.0/16 dev shz-br0 proto kernel scope link src 10.199.0.1 linkdown`, ufwState: "Status: active"}
	f.install(t)
	got, err := ProviderGatewayAllow("/unused", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "present" || got.Network != "10.199.0.0/16" || got.Interface != "shz-br0" {
		t.Fatalf("shz-br0 not accepted: %+v", got)
	}
}

// TestGatewayRefusesPhysicalInterfaceRoute guards the sharper edge: a bind
// on the host's LAN interface has a perfectly good proto-kernel scope-link
// route — but a physical NIC is not a bridge, and admitting its subnet
// would widen exposure past the container network.
func TestGatewayRefusesPhysicalInterfaceRoute(t *testing.T) {
	f := &gatewayFixture{bind: "10.0.0.7", routes: bridgeRoutes,
		bridgeLinks: `2: docker0: <BROADCAST> mtu 1500`, ufwState: "Status: active"}
	f.install(t)
	got, err := ProviderGatewayAllow("/unused", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "refused" || len(f.allowRecs) != 0 {
		t.Fatalf("physical NIC accepted: %+v edits=%v", got, f.allowRecs)
	}
}

func TestGatewayAllowRefusesLoopbackBind(t *testing.T) {
	f := &gatewayFixture{bind: "127.0.0.1", ufwState: "Status: active"}
	f.install(t)
	got, err := ProviderGatewayAllow("/unused", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "refused" || len(f.allowRecs) != 0 {
		t.Fatalf("state=%s edits=%v; want refused with zero edits", got.State, f.allowRecs)
	}
}

func TestGatewayAllowRefusesUnbackedSource(t *testing.T) {
	// a public bind with no bridge network containing it: the source would
	// have to be guessed, which the surface never does.
	f := &gatewayFixture{bind: "203.0.113.9", routes: bridgeRoutes, ufwState: "Status: active"}
	f.install(t)
	got, err := ProviderGatewayAllow("/unused", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "refused" || len(f.allowRecs) != 0 {
		t.Fatalf("state=%s edits=%d; want refused with zero edits", got.State, len(f.allowRecs))
	}
	if !strings.Contains(got.Detail, "guessed") {
		t.Fatalf("detail = %q; want the no-guess wording", got.Detail)
	}
}

func TestGatewayAllowWithoutUfwPrintsManualPath(t *testing.T) {
	f := &gatewayFixture{bind: "10.199.0.1", routes: bridgeRoutes, ufwState: ""}
	f.install(t)
	got, err := ProviderGatewayAllow("/unused", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "refused" || len(f.allowRecs) != 0 {
		t.Fatalf("state=%s edits=%v; want refused with zero edits", got.State, f.allowRecs)
	}
	joined := strings.Join(got.SuggestedCmd, " ")
	if !strings.HasPrefix(joined, "iptables -I INPUT 1 -s 10.199.0.0/24 -d 10.199.0.1 -p tcp") ||
		!strings.Contains(joined, providerGatewayTag) {
		t.Fatalf("manual path = %q; want the exact tagged iptables form", joined)
	}
}

func TestGatewayAllowShapeAndIdempotence(t *testing.T) {
	f := &gatewayFixture{bind: "10.199.0.1", routes: bridgeRoutes, ufwState: "Status: active"}
	f.install(t)
	got, err := ProviderGatewayAllow("/unused", 8443)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "present" || len(f.allowRecs) != 1 {
		t.Fatalf("state=%s edits=%v", got.State, f.allowRecs)
	}
	joined := strings.Join(f.allowRecs[0], " ")
	want := "allow from 10.199.0.0/24 to 10.199.0.1 proto tcp to 8443 comment " + providerGatewayTag
	if joined != want {
		t.Fatalf("rule = %q; want %q", joined, want)
	}
	// the numbered listing now carries the rule: the second allow is a
	// proven no-op.
	f.numbered = "[ 1] ufw allow from 10.199.0.0/24 to 10.199.0.1 proto tcp to 8443 comment " + providerGatewayTag
	again, err := ProviderGatewayAllow("/unused", 8443)
	if err != nil {
		t.Fatal(err)
	}
	if again.State != "present" || len(f.allowRecs) != 1 || !strings.Contains(again.Detail, "nothing changed") {
		t.Fatalf("idempotence broken: state=%s edits=%d detail=%q", again.State, len(f.allowRecs), again.Detail)
	}
}

func TestGatewayAllowForwardsEditFailure(t *testing.T) {
	f := &gatewayFixture{bind: "10.199.0.1", routes: bridgeRoutes, ufwState: "Status: active", failAllow: true}
	f.install(t)
	if _, err := ProviderGatewayAllow("/unused", 0); err == nil {
		t.Fatal("a rejected ufw edit must surface as an error")
	}
}

func TestGatewayRevokeTouchesOnlyTaggedRules(t *testing.T) {
	f := &gatewayFixture{bind: "10.199.0.1", routes: bridgeRoutes, ufwState: "Status: active",
		numbered: `[ 1] ALLOW IN Anywhere                     # operator rule
[ 2] ALLOW IN 10.199.0.0/24 -> 10.199.0.1:8443/tcp comment: ` + providerGatewayTag + `
[ 3] DENY IN 172.17.0.0/16                  # operator rule
[ 4] ALLOW IN 10.199.0.0/24 -> 10.199.0.1/tcp comment: ` + providerGatewayTag}
	f.install(t)
	got, err := ProviderGatewayRevoke("/unused")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "absent" {
		t.Fatalf("state=%s", got.State)
	}
	if len(f.delRecs) != 2 || f.delRecs[0] != 4 || f.delRecs[1] != 2 {
		t.Fatalf("deletions=%v; want exactly [4 2] (tagged only, high first)", f.delRecs)
	}
}

func TestGatewayRevokeNoopWhenUntagged(t *testing.T) {
	f := &gatewayFixture{bind: "10.199.0.1", routes: bridgeRoutes, ufwState: "Status: active",
		numbered: "[ 1] ALLOW IN 10.199.0.0/24 -> 10.199.0.1/tcp  # a lookalike without the tag"}
	f.install(t)
	got, err := ProviderGatewayRevoke("/unused")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "absent" || len(f.delRecs) != 0 {
		t.Fatalf("state=%s deletions=%v; want an untouched no-op", got.State, f.delRecs)
	}
}

func TestGatewayStatusTriStates(t *testing.T) {
	// not-applicable on loopback
	f := &gatewayFixture{bind: "::1", ufwState: "Status: active"}
	f.install(t)
	if got, _ := ProviderGatewayStatusOf("/unused", 0); got.State != "not-applicable" {
		t.Fatalf("loopback status=%s", got.State)
	}
	// absent under active ufw without the rule
	f2 := &gatewayFixture{bind: "10.199.0.1", routes: bridgeRoutes, ufwState: "Status: active", numbered: "[ 1] x\n"}
	f2.install(t)
	got, err := ProviderGatewayStatusOf("/unused", 0)
	if err != nil || got.State != "absent" {
		t.Fatalf("state=%s err=%v", got.State, err)
	}
	// present
	f2.numbered = "[ 1] ufw allow from 10.199.0.0/24 to 10.199.0.1 proto tcp comment " + providerGatewayTag
	if got, _ := ProviderGatewayStatusOf("/unused", 0); got.State != "present" {
		t.Fatalf("state=%s", got.State)
	}
}

func TestGatewayCorruptConfigRefuses(t *testing.T) {
	f := &gatewayFixture{bind: "not-an-ip", bindErr: fmt.Errorf("config is corrupt")}
	f.install(t)
	if _, err := ProviderGatewayAllow("/unused", 0); err == nil {
		t.Fatal("a corrupt bind read must fail closed")
	}
}

func TestGatewayNonCanonicalBindIsRefusedNotGuessed(t *testing.T) {
	// Whatever net.ParseIP's tolerance is for leading-zero text, the
	// surface must either carry ONLY canonical text into a rule or refuse
	// outright — never forward the raw config string.
	f := &gatewayFixture{bind: "10.199.000.001", routes: bridgeRoutes, ufwState: "Status: active"}
	f.install(t)
	got, err := ProviderGatewayAllow("/unused", 0)
	if err != nil || len(f.allowRecs) == 0 {
		if err == nil && got.State != "refused" {
			t.Fatalf("neither refusal nor error: %+v err=%v", got, err)
		}
		return // refused path: legal, nothing written
	}
	joined := strings.Join(f.allowRecs[0], " ")
	if strings.Contains(joined, "000") {
		t.Fatalf("rule carried raw non-canonical bind text: %q", joined)
	}
	if !strings.Contains(joined, "to 10.199.0.1 ") {
		t.Fatalf("rule did not carry the canonical bind: %q", joined)
	}
}
