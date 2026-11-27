//go:build !test_fake_routing

package main

import (
	"bytes"
	"strings"
	"testing"
)

// The gateway shell is the operator's explicit surface: it parses in the
// standard culler position, answers status for an unprovisioned home with
// the honest not-applicable state, and refuses unknown actions as usage.
func TestCullerGatewayShell(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	code := runCuller([]string{"gateway", "status", "--mc-home", home, "--no-color"}, nil, &out, &out)
	if code != 0 {
		t.Fatalf("status code = %d: %s", code, out.String())
	}
	text := out.String()
	if !strings.Contains(text, "not-applicable") && !strings.Contains(text, "absent") {
		t.Fatalf("status output lacks the state panel: %s", text)
	}
	out.Reset()
	if code := runCuller([]string{"gateway", "explode", "--mc-home", home, "--no-color"}, nil, &out, &out); code != 2 {
		t.Fatalf("unknown action: code = %d want 2: %s", code, out.String())
	}
	// allow against the default loopback bind must REFUSE without touching
	// anything: exit 1, state refused, in the operator's own words.
	out.Reset()
	if code := runCuller([]string{"gateway", "allow", "--mc-home", home, "--no-color"}, nil, &out, &out); code != 1 {
		t.Fatalf("loopback allow: code = %d want 1: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "refused") || !strings.Contains(out.String(), "loopback") {
		t.Fatalf("loopback allow output: %s", out.String())
	}
}
