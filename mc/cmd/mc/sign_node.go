package main

import (
	"errors"
	"fmt"
	"io"

	"mc/signnode"
)

// runPrivateSignNode is the transitional signing node (D06, O11 ruling
// P4/P5): one operation per invocation, all state on the disk ledger under
// the canonical MC_HOME. The node never opens the spine. It is a private
// host-local command and is not listed on the agent surface. The request
// arrives either on stdin or, for the verb's file handoff, via
// --request <path> (a staged 0600 file under the node home).
func runPrivateSignNode(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	requestPath := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--request" {
			requestPath = args[i+1]
		}
	}
	home, err := configuredCanonicalHome()
	if err != nil {
		fmt.Fprintln(stderr, "mc: sign-node:", err)
		return 2
	}
	var serveErr error
	if requestPath != "" {
		serveErr = signnode.ServeFile(home, requestPath, stdout)
	} else {
		serveErr = signnode.Serve(home, stdin, stdout)
	}
	if serveErr != nil {
		if errors.Is(serveErr, signnode.ErrUsage) {
			fmt.Fprintln(stderr, "mc: sign-node:", serveErr)
			return 2
		}
		fmt.Fprintln(stderr, "mc: sign-node:", serveErr)
		return 1
	}
	return 0
}
