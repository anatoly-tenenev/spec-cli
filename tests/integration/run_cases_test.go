package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	integrationharness "github.com/anatoly-tenenev/spec-cli/tests/integration/internal/harness"
)

type integrationCase = integrationharness.Case

func TestValidateCases(t *testing.T) {
	runCommandCases(t, "validate")
}

func TestQueryCases(t *testing.T) {
	runCommandCases(t, "query")
}

func TestGetCases(t *testing.T) {
	runCommandCases(t, "get")
}

func TestAddCases(t *testing.T) {
	runCommandCases(t, "add")
}

func TestUpdateCases(t *testing.T) {
	runCommandCases(t, "update")
}

func TestDeleteCases(t *testing.T) {
	runCommandCases(t, "delete")
}

func TestVersionCases(t *testing.T) {
	runCommandCases(t, "version")
}

func TestSchemaCases(t *testing.T) {
	runCommandCases(t, "schema")
}

func runCommandCases(t *testing.T, command string) {
	t.Helper()

	caseRoot := filepath.Join("cases", command)
	caseDirs, err := listCommandCaseDirs(caseRoot)
	if err != nil {
		t.Fatalf("list %s case directories: %v", command, err)
	}

	for _, caseDir := range caseDirs {
		testCase, err := loadCase(caseDir)
		if err != nil {
			t.Fatalf("load case %s: %v", caseDir, err)
		}
		if err := validateCaseNaming(command, caseDir, testCase); err != nil {
			t.Fatalf("validate case naming %s: %v", caseDir, err)
		}

		tc := testCase
		t.Run(tc.ID, func(t *testing.T) {
			integrationharness.RunCase(t, caseDir, tc)
		})
	}
}

func listCommandCaseDirs(caseRoot string) ([]string, error) {
	groupEntries, err := os.ReadDir(caseRoot)
	if err != nil {
		return nil, err
	}

	groupNames := make([]string, 0, len(groupEntries))
	for _, entry := range groupEntries {
		if entry.IsDir() {
			groupNames = append(groupNames, entry.Name())
		}
	}
	sort.Strings(groupNames)

	caseDirs := make([]string, 0)
	for _, groupName := range groupNames {
		groupDir := filepath.Join(caseRoot, groupName)
		caseEntries, err := os.ReadDir(groupDir)
		if err != nil {
			return nil, fmt.Errorf("read group directory %s: %w", groupDir, err)
		}

		caseNames := make([]string, 0, len(caseEntries))
		for _, entry := range caseEntries {
			if entry.IsDir() {
				caseNames = append(caseNames, entry.Name())
			}
		}
		sort.Strings(caseNames)

		for _, caseName := range caseNames {
			caseDirs = append(caseDirs, filepath.Join(groupDir, caseName))
		}
	}

	return caseDirs, nil
}

func loadCase(caseDir string) (integrationCase, error) {
	return integrationharness.LoadCase(caseDir)
}

func validateCaseNaming(command string, caseDir string, testCase integrationCase) error {
	caseName := filepath.Base(caseDir)
	caseParts := strings.SplitN(caseName, "_", 3)
	if len(caseParts) != 3 {
		return fmt.Errorf("case directory must match <NNNN>_<ok|err>_<case-id>, got %q", caseName)
	}

	caseNumber := caseParts[0]
	if len(caseNumber) != 4 || !isDigits(caseNumber) {
		return fmt.Errorf("case directory must start with 4-digit number, got %q", caseName)
	}

	outcome := caseParts[1]
	switch outcome {
	case "ok":
		if testCase.Expect.ExitCode != 0 {
			return fmt.Errorf("case outcome prefix %q requires exit_code 0, got %d", outcome, testCase.Expect.ExitCode)
		}
	case "err":
		if testCase.Expect.ExitCode == 0 {
			return fmt.Errorf("case outcome prefix %q requires non-zero exit_code", outcome)
		}
	default:
		return fmt.Errorf("case outcome prefix must be ok|err, got %q", outcome)
	}

	format, err := caseOutputFormat(testCase.Args)
	if err != nil {
		return err
	}
	if format == "json" && !strings.HasSuffix(caseName, "_json") {
		return fmt.Errorf("case directory for --format json must end with _json, got %q", caseName)
	}

	groupName := filepath.Base(filepath.Dir(caseDir))
	groupParts := strings.SplitN(groupName, "_", 2)
	if len(groupParts) != 2 || len(groupParts[0]) != 2 || !isDigits(groupParts[0]) {
		return fmt.Errorf("group directory must match <GG>_<group-name>, got %q", groupName)
	}

	expectedID := fmt.Sprintf("%s_%s_%s", command, groupParts[0], caseName)
	if testCase.ID != expectedID {
		return fmt.Errorf("case id mismatch: expected %q, got %q", expectedID, testCase.ID)
	}

	if testCase.Command != command {
		return fmt.Errorf("case command mismatch: expected %q, got %q", command, testCase.Command)
	}

	return nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, symbol := range value {
		if symbol < '0' || symbol > '9' {
			return false
		}
	}
	return true
}

func caseOutputFormat(args []string) (string, error) {
	for idx := 0; idx < len(args); idx++ {
		if args[idx] != "--format" {
			continue
		}
		if idx+1 >= len(args) {
			return "", fmt.Errorf("case args: --format requires a value")
		}

		format := args[idx+1]
		switch format {
		case "json":
			return format, nil
		default:
			return "", fmt.Errorf("case args: unsupported --format value %q", format)
		}
	}

	return "json", nil
}
