package substrate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scanActivityWriters enumerates every activity-row INSERT in the module's
// non-test sources: the marker `INSERT INTO activity` at a word boundary,
// so the mapping table's own inserts never match. The result carries
// repo-relative file:line positions. Any read or walk failure is returned
// as an error: the gate refuses to pass on a spine of evidence it could
// not collect (B05 fail-closed discipline).
func scanActivityWriters(root string) ([]string, error) {
	var sites []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for offset := 0; ; {
				at := strings.Index(line[offset:], "INSERT INTO activity")
				if at < 0 {
					break
				}
				start := offset + at
				end := start + len("INSERT INTO activity")
				if end < len(line) {
					next := line[end]
					if next == '_' || next >= 'a' && next <= 'z' || next >= 'A' && next <= 'Z' || next >= '0' && next <= '9' {
						offset = end
						continue
					}
				}
				sites = append(sites, fmt.Sprintf("%s:%d", rel, i+1))
				offset = end
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sites, nil
}

// activityModuleRoot locates the mc module root from this test file
// (mc/substrate/<this file> ⇒ mc).
func activityModuleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("the coverage gate cannot locate its own source file")
	}
	root := filepath.Dir(filepath.Dir(thisFile))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("the coverage gate found no module root above %s: %v", root, err)
	}
	return root
}

// CG1: the coverage direction — every activity-row INSERT in the module is
// the central append. A new write site that bypasses AppendActivity turns
// this gate red, and the gate is the ticket's named promise (B10: 新写点不
// 走中央 API 必红).
func TestActivityCoverageGateOnlyTheCentralAppendWrites(t *testing.T) {
	root := activityModuleRoot(t)
	sites, err := scanActivityWriters(root)
	if err != nil {
		t.Fatalf("enumerate activity writers: %v", err)
	}
	if len(sites) == 0 {
		t.Fatal("the enumerator found no activity writer at all; a parse or walk anomaly is a gate failure, not a pass")
	}
	for _, site := range sites {
		if !strings.HasPrefix(site, filepath.Join("substrate", "activity_chain.go")) {
			t.Fatalf("bypassing activity writer at %s; the central append is the only permitted writer", site)
		}
	}
}

// CG2: the phantom direction — the exemption is one statement, not one
// file. The central implementation keeps exactly one activity-row INSERT;
// if the append is ever gutted or duplicated, the gate refuses.
func TestActivityCoverageGateCentralAppendHoldsItsSingleSite(t *testing.T) {
	root := activityModuleRoot(t)
	sites, err := scanActivityWriters(root)
	if err != nil {
		t.Fatalf("enumerate activity writers: %v", err)
	}
	var mine []string
	for _, site := range sites {
		if strings.HasPrefix(site, filepath.Join("substrate", "activity_chain.go")) {
			mine = append(mine, site)
		}
	}
	if len(mine) != 1 {
		t.Fatalf("activity_chain.go holds %d activity writers (%v); want exactly the one inside AppendActivity", len(mine), mine)
	}
}

// CG3: the enumerator's own negative control on a fixture tree: a bypassing
// writer is reported, the mapping-table insert is not, and the boundary
// rule holds for suffix letters (activity2, activity_x).
func TestActivityCoverageGateEnumeratorDetectsBypass(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pkg/bypass.go", "package pkg\n\nconst q = `INSERT INTO activity (a) VALUES (1)`\n")
	write("pkg/mapping.go", "package pkg\n\nconst q = `INSERT INTO activity_chain_map (a) VALUES (1)`\n")
	write("pkg/suffix.go", "package pkg\n\nconst q = `INSERT INTO activity2`\nconst r = `INSERT INTO activity_x`\n")
	write("pkg/central.go", "package pkg\n\nconst q = `INSERT INTO activity\n\t(x) VALUES (1)`\n")
	write("pkg/ignored_test.go", "package pkg\n\nconst q = `INSERT INTO activity (a) VALUES (1)`\n")
	sites, err := scanActivityWriters(root)
	if err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	got := strings.Join(sites, ",")
	if strings.Contains(got, "mapping.go") || strings.Contains(got, "suffix.go") || strings.Contains(got, "ignored_test.go") {
		t.Fatalf("the enumerator matched a non-writer: %s", got)
	}
	if len(sites) != 2 {
		t.Fatalf("fixture sites = %v; want the bypass plus the central (2)", sites)
	}
	var sawBypass, sawCentral bool
	for _, site := range sites {
		sawBypass = sawBypass || strings.HasPrefix(site, filepath.Join("pkg", "bypass.go"))
		sawCentral = sawCentral || strings.HasPrefix(site, filepath.Join("pkg", "central.go"))
	}
	if !sawBypass || !sawCentral {
		t.Fatalf("fixture sites = %v; want bypass.go and central.go", sites)
	}
}
