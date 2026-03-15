package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commandbus"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/output/errormap"
	"github.com/anatoly-tenenev/spec-cli/internal/output/jsonwriter"
)

type App struct {
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
	bus    *commandbus.Bus
}

func NewApp(stdout, stderr io.Writer, now func() time.Time) *App {
	bus := commandbus.New()
	bus.Register("validate", validate.NewHandler())
	bus.Register("query", query.NewHandler())
	bus.Register("get", get.NewHandler())
	bus.Register("add", add.NewHandler(now))
	bus.Register("update", update.NewHandler(now))
	bus.Register("delete", delete.NewHandler())

	return &App{
		stdout: stdout,
		stderr: stderr,
		now:    now,
		bus:    bus,
	}
}

func (a *App) Run(ctx context.Context, args []string) int {
	globalOpts, commandName, commandArgs, parseErr := parseGlobalOptions(args)
	if parseErr != nil {
		a.writeError(parseErr)
		return parseErr.ExitCode
	}

	result, runErr := a.bus.Dispatch(ctx, requests.Command{
		Name:   commandName,
		Args:   commandArgs,
		Global: globalOpts,
	})
	if runErr != nil {
		a.writeError(runErr)
		return runErr.ExitCode
	}

	if err := a.writeSuccess(result); err != nil {
		internalErr := domainerrors.New(
			domainerrors.CodeInternalError,
			fmt.Sprintf("failed to render output: %v", err),
			nil,
		)
		a.writeError(internalErr)
		return internalErr.ExitCode
	}

	if result.ExitCode != 0 {
		return result.ExitCode
	}

	return 0
}

func (a *App) writeSuccess(result responses.CommandOutput) error {
	return jsonwriter.New(a.stdout).Write(result.JSON)
}

func (a *App) writeError(appErr *domainerrors.AppError) {
	state := errormap.ResultStateForCode(appErr.Code)
	errorPayload := map[string]any{
		"code":      appErr.Code,
		"message":   appErr.Message,
		"exit_code": appErr.ExitCode,
	}
	if len(appErr.Details) > 0 {
		errorPayload["details"] = appErr.Details
	}

	payload := map[string]any{
		"result_state": state,
		"error":        errorPayload,
	}
	if err := jsonwriter.New(a.stdout).Write(payload); err != nil {
		_, _ = fmt.Fprintf(a.stderr, "failed to write json error: %v\n", err)
	}
}
