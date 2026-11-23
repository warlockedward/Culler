//go:build docker_e2e && !darwin && !linux

package e2e

// e2eTempRoot is empty on this platform: the bind-mount and file-sharing
// behaviour the lanes depend on is only verified against Docker Desktop and the
// Linux engine, so a run skips rather than mounts against an unknown layout.
const e2eTempRoot = ""
