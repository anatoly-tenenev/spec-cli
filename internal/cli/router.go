package cli

func isSupportedCommand(name string) bool {
	switch name {
	case "validate", "query", "get", "add", "update", "delete", "version":
		return true
	default:
		return false
	}
}
