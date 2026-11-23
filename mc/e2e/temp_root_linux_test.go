//go:build docker_e2e && linux

package e2e

// e2eTempRoot hosts MC_HOME and the scratch spine for a run.
//
// Linux has no file-sharing VM between the daemon and the host, so any local
// path is bind-mountable. `/private/tmp` is a Darwin path that does not exist
// here; `/tmp` is a plain directory rather than a symlink.
const e2eTempRoot = "/tmp"
