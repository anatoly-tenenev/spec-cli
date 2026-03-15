package cli

func isSupportedCommand(name string) bool {
	switch name {
	case "validate", "query", "get", "add", "update", "delete":
		return true
	default:
		return false
	}
}
