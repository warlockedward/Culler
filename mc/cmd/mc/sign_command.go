package main

import (
	"io"

	"mc/verbs"
)

// commandSign is the anchor signing state machine's CLI surface (D06):
// request creation, execution against the transitional node, reconciliation,
// fencing cancel, and supersession. Host-local command, per-action system
// spine call, so the helper delegation path is the same proven one.
func commandSign(args []string, stdin io.Reader) (any, error) {
	if len(args) != 2 {
		return nil, verbs.Usagef("sign: mc sign <create|execute|reconcile|cancel|supersede>")
	}
	switch args[0] {
	case "create":
		var a verbs.SignRequestCreateRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("sign-create", a)
	case "execute":
		var a verbs.SignRequestExecuteRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("sign-execute", a)
	case "reconcile":
		var a verbs.SignRequestReconcileRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("sign-reconcile", a)
	case "cancel":
		var a verbs.SignRequestCancelRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("sign-cancel", a)
	case "supersede":
		var a verbs.SignRequestSupersedeRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("sign-supersede", a)
	}
	return nil, verbs.Usagef("sign: mc sign <create|execute|reconcile|cancel|supersede>")
}
