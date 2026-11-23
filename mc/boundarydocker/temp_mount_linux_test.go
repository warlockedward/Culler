//go:build docker_boundary && linux

package boundarydocker

// tempMountRoot is the parent for directories that become Docker bind
// mountpoints.
//
// `/private/tmp` is a Darwin path that does not exist here. `/tmp` is a real
// directory rather than a symlink on Linux, and the provenance xattr that makes
// a Darwin teardown resist removal is a Docker Desktop artifact, so the
// directory removes normally.
const tempMountRoot = "/tmp"
