// Command spec-cli is the schema-aware CLI for working with specification
// documents. This package is the process boundary and nothing more: it wires
// stdout, stderr and the clock into the application and turns the returned
// code into an exit status.
//
// SPEC_CLI_FIXED_NOW_UTC pins the clock. Commands stamp createdDate and
// updatedDate from it, so without a fixed clock the documents the integration
// suite writes could not be compared against expected output.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/cli"
)

const fixedNowEnv = "SPEC_CLI_FIXED_NOW_UTC"

func main() {
	nowFn, err := resolveNow()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(5)
	}

	app := cli.NewApp(os.Stdout, os.Stderr, nowFn)
	os.Exit(app.Run(context.Background(), os.Args[1:]))
}

func resolveNow() (func() time.Time, error) {
	value := strings.TrimSpace(os.Getenv(fixedNowEnv))
	if value == "" {
		return time.Now, nil
	}

	parsed, err := parseFixedNow(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be RFC3339 or YYYY-MM-DD: %w", fixedNowEnv, err)
	}
	return func() time.Time { return parsed }, nil
}

func parseFixedNow(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty value")
	}

	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("invalid time format")
}
