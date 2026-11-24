package deployment

// UnitLabel returns the launchd label of one supervised unit.
func UnitLabel(names Names, unit string) string {
	return "com.s.shorizon." + names.Suffix + "." + unit
}
