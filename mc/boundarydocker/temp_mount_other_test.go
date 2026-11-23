//go:build docker_boundary && !darwin && !linux

package boundarydocker

// tempMountRoot is empty on this platform. The bind-mount behaviour these lanes
// depend on is only verified against Docker Desktop and the Linux engine, so an
// empty root makes the shared helper skip rather than run against an unknown
// host layout or fail with a missing path.
const tempMountRoot = ""
