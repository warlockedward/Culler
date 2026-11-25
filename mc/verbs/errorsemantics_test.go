package verbs

// The write-verb error-semantics registry gate. Every storage path of a write
// verb carries a six-dimension registration (busy behavior, CAS-loser result,
// stage-split retryability, side-effect uncertainty, exit code, JSON error
// class). A write path missing from the registry, or a registry entry whose
// path no longer exists, stops the suite.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

type semanticsRetryStage struct {
	Stage   string `json:"stage"`
	Verdict string `json:"verdict"`
}

type semanticsPath struct {
	Verb            string                `json:"verb"`
	Entrypoint      string                `json:"entrypoint"`
	BCasLoserResult string                `json:"b_cas_loser_result"`
	CRetryability   []semanticsRetryStage `json:"c_retryability"`
	DSideEffect     string                `json:"d_side_effect_uncertainty"`
	EExitCode       int                   `json:"e_exit_code"`
	FErrorClasses   []string              `json:"f_json_error_classes"`
}

type semanticsRegistry struct {
	ProtocolVersion int             `json:"protocol_version"`
	GlobalA         string          `json:"a_busy_behavior"`
	GlobalE         string          `json:"e_exit_codes"`
	Paths           []semanticsPath `json:"paths"`
}

func loadSemanticsRegistry(t *testing.T) semanticsRegistry {
	t.Helper()
	data, err := os.ReadFile("error_semantics_registry.json")
	if err != nil {
		t.Fatalf("read error semantics registry: %v", err)
	}
	var reg semanticsRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("parse error semantics registry: %v", err)
	}
	return reg
}

var mainGoPattern = regexp.MustCompile(`\.\./cmd/mc/main\.go`)

// readMainDispatchSource loads the CLI dispatch source the gate parses. The
// test runs with CWD = the verbs package directory.
func readMainDispatchSource(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "cmd", "mc", "main.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dispatch source %s: %v", path, err)
	}
	return string(data)
}

// functionBody returns the text of each top-level function in src.
func functionBodies(src string) map[string]string {
	bodies := map[string]string{}
	lines := splitLines(src)
	starts := []int{}
	names := []string{}
	funcRe := regexp.MustCompile(`^func (\w+)\(`)
	for i, ln := range lines {
		if m := funcRe.FindStringSubmatch(ln); m != nil {
			starts = append(starts, i)
			names = append(names, m[1])
		}
	}
	for k := range starts {
		end := len(lines)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		bodies[names[k]] = joinLines(lines[starts[k]:end])
	}
	return bodies
}

func splitLines(src string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			out = append(out, src[start:i])
			start = i + 1
		}
	}
	if start < len(src) {
		out = append(out, src[start:])
	}
	return out
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// caseSegments splits one function body into its switch-case segments.
// Lines before the first case form the preamble segment; a function without
// any case line yields its whole body as the single segment, so a verb call
// in a plain withSpine closure is still attributed.
func caseSegments(body string) []string {
	lines := splitLines(body)
	segs := []string{}
	current := []string{}
	caseRe := regexp.MustCompile(`^\s*case .+:`)
	for _, ln := range lines {
		if caseRe.MatchString(ln) {
			if joinedHasContent(current) {
				segs = append(segs, joinLines(current))
			}
			current = []string{ln}
			continue
		}
		current = append(current, ln)
	}
	if joinedHasContent(current) {
		segs = append(segs, joinLines(current))
	}
	return segs
}

func joinedHasContent(lines []string) bool {
	for _, l := range lines {
		if l != "" {
			return true
		}
	}
	return false
}

// enumerateWritePaths returns the set of verbs.<Func> names invoked inside a
// case segment that opens the spine for writing (withSpine). Segments that
// only open read-only spines (withSpineRead) contribute nothing: a write
// routed through the read path fails loudly by construction.
func enumerateWritePaths(src string) map[string]bool {
	paths := map[string]bool{}
	verbCallRe := regexp.MustCompile(`verbs\.([A-Z]\w*)\(db\b`)
	for _, body := range functionBodies(src) {
		if !containsWriteSpineOpen(body) {
			continue
		}
		for _, seg := range caseSegments(body) {
			if !containsWriteSpineOpen(seg) {
				continue
			}
			for _, m := range verbCallRe.FindAllStringSubmatch(seg, -1) {
				paths[m[1]] = true
			}
		}
	}
	return paths
}

func containsWriteSpineOpen(text string) bool {
	// "withSpine(" never matches "withSpineRead(": the read helper's name
	// carries "Read" before the opening parenthesis.
	return regexp.MustCompile(`withSpine\(`).MatchString(text)
}

// G1 anchor sanity: these write paths exist in every release of this CLI. If
// the parser reports none of them, the source changed shape and the
// enumeration is untrustworthy — fail closed instead of passing on an empty
// or shrunken set.
var writePathAnchors = []string{
	"TaskAdd", "TaskBlock", "DispatchWithObserver", "Heartbeat",
	"Complete", "ArbiterDecide", "OutboxAck", "CollieClaim",
}

func TestWritePathEnumerationIsSound(t *testing.T) {
	src := readMainDispatchSource(t)
	paths := enumerateWritePaths(src)
	for _, anchor := range writePathAnchors {
		if !paths[anchor] {
			t.Fatalf("enumeration anchor %s missing from the write-path set; the parser no longer matches the dispatch source shape:\n%s", anchor, sortedPathList(paths))
		}
	}
	if len(paths) < len(writePathAnchors) {
		t.Fatalf("write-path set shrank to %d; fail closed", len(paths))
	}
	t.Logf("write-path set (%d): %s", len(paths), sortedPathList(paths))
}

func sortedPathList(paths map[string]bool) string {
	names := []string{}
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += " "
		}
		out += n
	}
	return out
}

// G1 (coverage direction): every enumerated write path is registered.
func TestEveryWritePathIsRegistered(t *testing.T) {
	reg := loadSemanticsRegistry(t)
	src := readMainDispatchSource(t)
	enumerated := enumerateWritePaths(src)
	missing := []string{}
	for name := range enumerated {
		if !registryHasPath(reg, name) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("write paths missing from the error-semantics registry (registration is a review refusal item): %s", sortedPathList(mapFromList(missing)))
	}
}

// G2 (phantom direction): every registry entry names a path that exists.
func TestRegistryHasNoPhantomPaths(t *testing.T) {
	reg := loadSemanticsRegistry(t)
	src := readMainDispatchSource(t)
	enumerated := enumerateWritePaths(src)
	phantom := []string{}
	for _, p := range reg.Paths {
		if !enumerated[p.Verb] {
			phantom = append(phantom, p.Verb)
		}
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Fatalf("registry entries naming write paths that no longer exist: %s", joinNames(phantom))
	}
}

// G3 (six-dimension completeness): no entry leaves a dimension empty, and
// retryability is split by side-effect stage, never a single blanket value.
func TestRegistryEntriesAreComplete(t *testing.T) {
	reg := loadSemanticsRegistry(t)
	if reg.ProtocolVersion != 1 {
		t.Fatalf("registry protocol_version = %d; want 1", reg.ProtocolVersion)
	}
	if reg.GlobalA == "" || reg.GlobalE == "" {
		t.Fatal("the global busy-behavior and exit-code facts must be registered once")
	}
	seen := map[string]bool{}
	for _, p := range reg.Paths {
		if seen[p.Verb] {
			t.Fatalf("registry lists %s twice", p.Verb)
		}
		seen[p.Verb] = true
		if p.Entrypoint == "" || p.BCasLoserResult == "" || p.DSideEffect == "" {
			t.Fatalf("path %s leaves a dimension empty", p.Verb)
		}
		if p.EExitCode != 0 && p.EExitCode != 1 && p.EExitCode != 2 {
			t.Fatalf("path %s exit code %d is outside the 0/1/2 contract", p.Verb, p.EExitCode)
		}
		if len(p.CRetryability) == 0 {
			t.Fatalf("path %s registered no retryability stage", p.Verb)
		}
		for _, s := range p.CRetryability {
			if s.Stage == "" || s.Verdict == "" {
				t.Fatalf("path %s carries an empty retryability stage", p.Verb)
			}
		}
		if len(p.FErrorClasses) == 0 {
			t.Fatalf("path %s registered no JSON error class", p.Verb)
		}
	}
}

// G4 (discriminability): two write paths that share exit code 1 must be
// distinguishable from each other's failure record — by their error classes,
// or by their CAS-loser result, or by their side-effect statement. The
// machine checks the recorded text; prose semantics stay a review duty.
func TestRejectedPathsStayDistinguishable(t *testing.T) {
	reg := loadSemanticsRegistry(t)
	byVerb := map[string]semanticsPath{}
	for _, p := range reg.Paths {
		byVerb[p.Verb] = p
	}
	names := []string{}
	for n := range byVerb {
		names = append(names, n)
	}
	sort.Strings(names)
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			a, b := byVerb[names[i]], byVerb[names[j]]
			if a.EExitCode != 1 || b.EExitCode != 1 {
				continue
			}
			if sameErrorClasses(a.FErrorClasses, b.FErrorClasses) &&
				a.BCasLoserResult == b.BCasLoserResult &&
				a.DSideEffect == b.DSideEffect {
				t.Fatalf("paths %s and %s share exit code 1 and are indistinguishable on (b), (d), and (f)", a.Verb, b.Verb)
			}
		}
	}
}

func sameErrorClasses(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	am := map[string]bool{}
	for _, x := range a {
		am[x] = true
	}
	for _, x := range b {
		if !am[x] {
			return false
		}
	}
	return true
}

func registryHasPath(reg semanticsRegistry, verb string) bool {
	for _, p := range reg.Paths {
		if p.Verb == verb {
			return true
		}
	}
	return false
}

func mapFromList(names []string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// N1 (refusal test, missing registration): removing one entry from the
// registry in memory must make the coverage check report the gap. This is
// the ticket's acceptance fact — absence of a registration is a refusal.
func TestMissingRegistrationIsRefused(t *testing.T) {
	reg := loadSemanticsRegistry(t)
	if len(reg.Paths) == 0 {
		t.Fatal("the registry holds no paths; the negative control has nothing to remove")
	}
	removed := reg.Paths[0].Verb
	trimmed := append([]semanticsPath{}, reg.Paths[1:]...)
	reg.Paths = trimmed

	enumerated := map[string]bool{removed: true}
	missing := []string{}
	for name := range enumerated {
		if !registryHasPath(reg, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) != 1 || missing[0] != removed {
		t.Fatalf("coverage check on the trimmed registry = %v; want exactly [%s]", missing, removed)
	}
}

// N2 (refusal test, new unregistered path): a write path added to the
// dispatch source but absent from the registry must be reported by the same
// coverage check — the shape by which future verbs are caught.
func TestNewUnregisteredPathIsRefused(t *testing.T) {
	reg := loadSemanticsRegistry(t)
	enumerated := map[string]bool{"TaskAdd": true, "FutureWriteVerb": true}
	missing := []string{}
	for name := range enumerated {
		if !registryHasPath(reg, name) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) == 0 {
		t.Fatal("coverage check passed over an unregistered write path; the gate does not bite")
	}
	found := false
	for _, m := range missing {
		if m == "FutureWriteVerb" {
			found = true
		}
	}
	if !found {
		t.Fatalf("coverage check = %v; must name the unregistered path", missing)
	}
}
