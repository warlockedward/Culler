package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"mc/deployment"
)

// The recovery drill is a host-only verb: it proves the encrypted backup and
// restore path end to end without ever writing the live spine.

func runRecoveryDrill(home string) (any, error) {
	canonical, err := deployment.CanonicalHome(home)
	if err != nil {
		return nil, err
	}
	image, err := configuredProductionImage()
	if err != nil {
		return nil, err
	}
	names, err := deployment.RuntimeNames(canonical)
	if err != nil {
		return nil, err
	}
	dockerVersion := ""
	if out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output(); err == nil {
		dockerVersion = strings.TrimSpace(string(out))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	result, err := deployment.RunRecoveryDrill(ctx, canonical, image, names.Volume, releaseBuildID, dockerVersion, nil)
	if err != nil {
		return result, fmt.Errorf("recovery drill: %w", err)
	}
	return result, nil
}
