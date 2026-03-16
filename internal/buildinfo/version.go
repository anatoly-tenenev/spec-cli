package buildinfo

import (
	"fmt"
	"strings"
)

// Version is set at build time via -ldflags.
// Default value is used for local/dev builds.
var Version = "dev"

func ResolveVersion() (string, error) {
	trimmed := strings.TrimSpace(Version)
	if trimmed == "" {
		return "", fmt.Errorf("build version is empty")
	}
	return trimmed, nil
}
