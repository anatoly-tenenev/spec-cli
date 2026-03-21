package integration_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestGlobalOptionsCases(t *testing.T) {
	caseDirs := collectGroupedCaseDirs(t, filepath.Join("cases", "global_options"), "global options")

	for _, caseDir := range caseDirs {
		tc, loadErr := loadCase(caseDir)
		if loadErr != nil {
			t.Fatalf("load case %s: %v", caseDir, loadErr)
		}

		testCase := tc
		t.Run(testCase.ID, func(t *testing.T) {
			runCase(t, caseDir, testCase)
		})
	}
}

func collectGroupedCaseDirs(t *testing.T, caseRoot string, label string) []string {
	t.Helper()

	groupEntries, err := os.ReadDir(caseRoot)
	if err != nil {
		t.Fatalf("read %s groups: %v", label, err)
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
		groupPath := filepath.Join(caseRoot, groupName)
		caseEntries, readErr := os.ReadDir(groupPath)
		if readErr != nil {
			t.Fatalf("read %s group %s: %v", label, groupName, readErr)
		}

		caseNames := make([]string, 0, len(caseEntries))
		for _, entry := range caseEntries {
			if entry.IsDir() {
				caseNames = append(caseNames, entry.Name())
			}
		}
		sort.Strings(caseNames)

		for _, caseName := range caseNames {
			caseDirs = append(caseDirs, filepath.Join(groupPath, caseName))
		}
	}

	return caseDirs
}
