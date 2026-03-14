package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var tempFilePattern = regexp.MustCompile(`\.spec-cli-add-[0-9]+\.tmp`)

type WorkspacePermission struct {
	Path string
	Mode string
}

func NormalizeResponseValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		for key, item := range typed {
			next[key] = NormalizeResponseValue(item)
		}
		return next
	case []any:
		next := make([]any, len(typed))
		for idx, item := range typed {
			next[idx] = NormalizeResponseValue(item)
		}
		return next
	case string:
		return normalizeDynamicReason(typed)
	default:
		return value
	}
}

func ApplyWorkspacePermissions(workspaceRoot string, permissions []WorkspacePermission) (func(), error) {
	type rollback struct {
		path string
		mode os.FileMode
	}

	rollbacks := make([]rollback, 0, len(permissions))
	for _, permission := range permissions {
		cleanPath := filepath.Clean(permission.Path)
		if cleanPath == "." || cleanPath == "" {
			return nil, fmt.Errorf("permission path must not be empty")
		}
		if filepath.IsAbs(cleanPath) {
			return nil, fmt.Errorf("permission path must be relative: %s", permission.Path)
		}

		targetPath := filepath.Join(workspaceRoot, cleanPath)
		relative, relErr := filepath.Rel(workspaceRoot, targetPath)
		if relErr != nil {
			return nil, fmt.Errorf("resolve permission path %s: %w", permission.Path, relErr)
		}
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("permission path escapes workspace: %s", permission.Path)
		}

		parsedMode, parseErr := strconv.ParseUint(permission.Mode, 8, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid permission mode %q: %w", permission.Mode, parseErr)
		}

		info, statErr := os.Stat(targetPath)
		if statErr != nil {
			return nil, fmt.Errorf("stat %s: %w", targetPath, statErr)
		}
		rollbacks = append(rollbacks, rollback{path: targetPath, mode: info.Mode().Perm()})

		if err := os.Chmod(targetPath, os.FileMode(parsedMode)); err != nil {
			return nil, fmt.Errorf("chmod %s to %s: %w", targetPath, permission.Mode, err)
		}
	}

	return func() {
		for _, item := range rollbacks {
			_ = os.Chmod(item.path, item.mode)
		}
	}, nil
}

func normalizeDynamicReason(value string) string {
	normalized := strings.ReplaceAll(value, "\\", "/")
	if idx := strings.Index(normalized, "/workspace/"); idx >= 0 {
		normalized = "<workspace>" + normalized[idx+len("/workspace"):]
	}
	normalized = tempFilePattern.ReplaceAllString(normalized, ".spec-cli-add-<tmp>.tmp")
	return normalized
}
