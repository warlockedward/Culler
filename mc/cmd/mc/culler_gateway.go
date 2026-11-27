//go:build !test_fake_routing

package main

import (
	"fmt"
	"strings"

	"mc/deployment"
	"mc/onboarding"
)

// cullerGatewayAction is the operator surface for the Linux provider gateway
// rule (A36). It exists because the operator never runs mc: the whole point
// of the surface is that a human decided to open the deployment's bridge
// path, so the human-facing command names the action and prints exactly
// what was changed or what the tool refused and why. It runs only on the
// operator's own host.
func cullerGatewayAction(action, home string, out *onboarding.Terminal, fail func(error, int) int) int {
	if !operatorHostScope() {
		return fail(fmt.Errorf("the provider gateway manages the operator's own host; run it there"), 2)
	}
	switch action {
	case "", "status":
		status, err := deployment.ProviderGatewayStatusOf(home, 0)
		if err != nil {
			return fail(err, 1)
		}
		renderGateway(out, status)
		return 0
	case "allow":
		status, err := deployment.ProviderGatewayAllow(home, 0)
		if err != nil {
			return fail(err, 1)
		}
		renderGateway(out, status)
		if status.State == "refused" {
			return 1
		}
		return 0
	case "revoke":
		status, err := deployment.ProviderGatewayRevoke(home)
		if err != nil {
			return fail(err, 1)
		}
		renderGateway(out, status)
		if status.State == "refused" {
			return 1
		}
		return 0
	default:
		return fail(fmt.Errorf("gateway action %q is unknown; use status, allow, or revoke", action), 2)
	}
}

func renderGateway(out *onboarding.Terminal, s deployment.ProviderGatewayStatus) {
	out.Panel("Provider gateway", []string{
		"state:   " + s.State,
		"bind:    " + s.Bind,
	})
	if s.Network != "" {
		out.Field("source", s.Network+" ("+s.Interface+")")
	}
	if s.Port > 0 {
		out.Field("port", fmt.Sprintf("%d", s.Port))
	}
	if s.Detail != "" {
		out.Literal(s.Detail)
	}
	if len(s.SuggestedCmd) > 0 {
		out.Literal(strings.Join(s.SuggestedCmd, " "))
	}
}
