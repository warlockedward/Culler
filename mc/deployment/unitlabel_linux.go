package deployment

// UnitLabel returns the systemd unit name stem of one supervised unit
// (the ".service" suffix is added by the install and controller paths).
func UnitLabel(names Names, unit string) string {
	return "shorizon-" + names.Suffix + "-" + unit
}
