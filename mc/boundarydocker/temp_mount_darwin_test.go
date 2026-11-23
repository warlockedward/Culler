//go:build docker_boundary && darwin

package boundarydocker

// tempMountRoot is the parent for directories that become Docker bind
// mountpoints.
//
// t.TempDir lives under `/var/folders`, and a directory there that has served
// as a nested bind mountpoint acquires a `com.apple.provenance` xattr from
// Docker Desktop which makes it resist `os.RemoveAll` with EPERM — so the test
// body passes and then the harness fails the test during teardown. Under
// `/private/tmp` the same directory removes cleanly.
const tempMountRoot = "/private/tmp"
