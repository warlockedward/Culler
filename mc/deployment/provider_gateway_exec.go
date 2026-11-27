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

func gatewayIPRoute() (string, error) {
	return runGateway(&exec.Cmd{Path: "ip", Args: []string{"ip", "-o", "route", "show"}})
}

func gatewayUfwState() (string, error) {
	return runGateway(&exec.Cmd{Path: "ufw", Args: []string{"ufw", "status"}})
}

func gatewayUfwNumbered() (string, error) {
	return runGateway(&exec.Cmd{Path: "ufw", Args: []string{"ufw", "status", "numbered"}})
}

// gatewayUfwAllow receives only vectors built by ruleSpec.
func gatewayUfwAllow(spec []string) error {
	out, err := runGateway(&exec.Cmd{Path: "ufw", Args: append([]string{"ufw"}, spec...)})
	if err != nil {
		return fmt.Errorf("ufw rejected the rule (output: %s): %w", strings.TrimSpace(out), err)
	}
	return nil
}

// gatewayUfwDelete renders the integer index through strconv only.
func gatewayUfwDelete(index int) error {
	out, err := runGateway(&exec.Cmd{Path: "ufw", Args: []string{"ufw", "--force", "delete", strconv.Itoa(index)}})
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
