package cli

func isSupportedCommand(name string) bool {
	switch name {
	case "validate", "query", "get", "add", "update":
		return true
	default:
		return false
	}
}
