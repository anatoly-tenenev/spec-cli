package cli

import (
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commandbus"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/help"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/version"
	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"
)

var appCommandCatalog = helpmodel.MustCatalog([]helpmodel.CommandSpec{
	help.HelpSpec(),
	query.HelpSpec(),
	get.HelpSpec(),
	add.HelpSpec(),
	update.HelpSpec(),
	delete.HelpSpec(),
	validate.HelpSpec(),
	version.HelpSpec(),
})

func commandCatalog() *helpmodel.Catalog {
	return appCommandCatalog
}

func supportedCommandNames() []string {
	return commandCatalog().Names()
}

func isSupportedCommand(name string) bool {
	return commandCatalog().Has(name)
}

func registerCommandHandlers(bus *commandbus.Bus, now func() time.Time) {
	catalog := commandCatalog()
	bus.Register("help", help.NewHandler(catalog))
	bus.Register("query", query.NewHandler())
	bus.Register("get", get.NewHandler())
	bus.Register("add", add.NewHandler(now))
	bus.Register("update", update.NewHandler(now))
	bus.Register("delete", delete.NewHandler())
	bus.Register("validate", validate.NewHandler())
	bus.Register("version", version.NewHandler())
}
