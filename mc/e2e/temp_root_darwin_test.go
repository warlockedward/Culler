//go:build docker_e2e && darwin

package e2e

// e2eTempRoot hosts MC_HOME and the scratch spine for a run.
//
// `/private/tmp` is shared with the Docker Desktop VM by default; `t.TempDir()`
// lands in `/var/folders`, which may not be, and then a bind mount silently
// presents an empty directory inside the container.
const e2eTempRoot = "/private/tmp"
