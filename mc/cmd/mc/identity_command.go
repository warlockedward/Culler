package main

import (
	"io"

	"mc/verbs"
)

// commandIdentity is the P4 data-access identity plane's CLI surface (D01/
// D03/D05): account bootstrap, session lifecycle, grant lifecycle, legacy
// actor mapping, and degraded operation authorization. It follows the
// connection/usage shape: host-local command, per-action system spine call,
// so the helper delegation path is the same proven one.
func commandIdentity(args []string, stdin io.Reader) (any, error) {
	if len(args) != 2 {
		return nil, verbs.Usagef("identity: mc identity <user-create|session-create|session-revoke|grant-issue|grant-revoke|legacy-map|degraded-grant-issue|degraded-grant-revoke>")
	}
	switch args[0] {
	case "user-create":
		var a verbs.IdentityUserCreateRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-user-create", a)
	case "session-create":
		var a verbs.IdentitySessionCreateRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-session-create", a)
	case "session-revoke":
		var a verbs.IdentitySessionRevokeRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-session-revoke", a)
	case "grant-issue":
		var a verbs.IdentityGrantIssueRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-grant-issue", a)
	case "grant-revoke":
		var a verbs.IdentityGrantRevokeRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-grant-revoke", a)
	case "legacy-map":
		var a verbs.IdentityLegacyMapRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-legacy-map", a)
	case "degraded-grant-issue":
		var a verbs.IdentityDegradedGrantIssueRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-degraded-grant-issue", a)
	case "degraded-grant-revoke":
		var a verbs.IdentityDegradedGrantRevokeRequest
		if err := decodeDiagnosticRequest(stdin, &a); err != nil {
			return nil, err
		}
		return systemSpineCall("identity-degraded-grant-revoke", a)
	}
	return nil, verbs.Usagef("identity: mc identity <user-create|session-create|session-revoke|grant-issue|grant-revoke|legacy-map|degraded-grant-issue|degraded-grant-revoke>")
}
