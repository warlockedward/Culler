package main

import (
	"io"

	"mc/verbs"
)

// commandUsage records provider usage facts on the spine: the raw event
// first (phase one, the durable input), then the derived observation
// (phase two, the identity-carrying fact). Both run through the system
// spine call path, the same shape as the connection observations.
func commandUsage(args []string, stdin io.Reader) (any, error) {
	if len(args) != 1 {
		return nil, verbs.Usagef("usage: mc usage <event|observation|pending>")
	}
	switch args[0] {
	case "event":
		var a verbs.UsageEventRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("usage-event", a)
	case "observation":
		var a verbs.UsageObservationRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("usage-observation", a)
	case "pending":
		var a verbs.UsagePendingRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("usage-pending", a)
	}
	return nil, verbs.Usagef("usage: mc usage <event|observation|pending>")
}
