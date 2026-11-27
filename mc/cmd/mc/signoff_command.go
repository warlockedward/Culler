package main

import (
	"io"

	"mc/verbs"
)

// commandSignoff is the signoffs CLI surface (D08): recording an approval
// or rejection for a specific subject. Host-local command, per-action
// system spine call, so the helper delegation path is the same proven one.
func commandSignoff(args []string, stdin io.Reader) (any, error) {
	if len(args) != 2 {
		return nil, verbs.Usagef("signoff: mc signoff <record>")
	}
	if args[0] == "record" {
		var a verbs.SignoffRecordRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("signoff-record", a)
	}
	return nil, verbs.Usagef("signoff: mc signoff <record>")
}
