package cli

func isSupportedCommand(name string) bool {
	switch name {
	case "validate", "query", "add", "update":
		return true
	default:
		return false
	}
}
