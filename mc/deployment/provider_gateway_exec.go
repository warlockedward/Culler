package deployment

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// This file is the ONLY place in the gateway surface that starts a program.
// The command shapes are explicit exec.Cmd literals: the Path is a string
// constant and the Arg vector is spelled out in place. The two parameterized
// edits take exactly one argument each, produced by the pure builders in
// provider_gateway.go — rule text re-rendered from net.ParseIP and
// net.ParseCIDR canonical output plus strconv-rendered ports, and rule
// indices from parsed integers. No shell is involved anywhere; each command
// runs to completion within the caller's CLI process lifetime.

// The host programs run at fixed FHS admin paths. Absolute paths are a
// correctness requirement, not style: the production mc binary is
// statically linked and a supervisor may run it without /usr/sbin on PATH
// — a bare name then fails LookPath on a host where the tool is installed
// (found on the real O3 machine). A host without iproute2 surfaces the
// route-read error; a host without ufw takes the refused-manual-command
// path.

func gatewayIPRoute() (string, error) {
	return runGateway(&exec.Cmd{Path: "/usr/sbin/ip", Args: []string{"ip", "-o", "route", "show"}})
}

func gatewayIPLinkBridges() (string, error) {
	return runGateway(&exec.Cmd{Path: "/usr/sbin/ip", Args: []string{"ip", "-o", "link", "show", "type", "bridge"}})
}

func gatewayUfwState() (string, error) {
	return runGateway(&exec.Cmd{Path: "/usr/sbin/ufw", Args: []string{"ufw", "status"}})
}

func gatewayUfwNumbered() (string, error) {
	return runGateway(&exec.Cmd{Path: "/usr/sbin/ufw", Args: []string{"ufw", "status", "numbered"}})
}

// gatewayUfwAllow receives only vectors built by ruleSpec.
func gatewayUfwAllow(spec []string) error {
	out, err := runGateway(&exec.Cmd{Path: "/usr/sbin/ufw", Args: append([]string{"ufw"}, spec...)})
	if err != nil {
		return fmt.Errorf("ufw rejected the rule (output: %s): %w", strings.TrimSpace(out), err)
	}
	return nil
}

// gatewayUfwDelete renders the integer index through strconv only.
func gatewayUfwDelete(index int) error {
	out, err := runGateway(&exec.Cmd{Path: "/usr/sbin/ufw", Args: []string{"ufw", "--force", "delete", strconv.Itoa(index)}})
	if err != nil {
		return fmt.Errorf("ufw rejected the deletion (output: %s): %w", strings.TrimSpace(out), err)
	}
	return nil
}

func runGateway(cmd *exec.Cmd) (string, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}
