package options

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	defaultLimit  = 100
	defaultOffset = 0
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{
		TypeFilters: []string{},
		Selects:     []string{},
		Sorts:       []model.SortTerm{},
		Limit:       defaultLimit,
		Offset:      defaultOffset,
	}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		if token == "--help" || token == "-h" {
			opts.Help = true
			continue
		}

		name, inlineValue, hasInlineValue := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown query option: %s", token),
				nil,
			)
		}

		switch name {
		case "--type":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--type value cannot be empty",
					nil,
				)
			}
			opts.TypeFilters = append(opts.TypeFilters, value)
			idx = nextIdx
		case "--where-json":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			if strings.TrimSpace(value) == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidQuery,
					"--where-json cannot be empty",
					nil,
				)
			}
			opts.WhereJSON = value
			idx = nextIdx
		case "--select":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--select value cannot be empty",
					nil,
				)
			}
			opts.Selects = append(opts.Selects, value)
			idx = nextIdx
		case "--sort":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			term, parseErr := parseSortTerm(value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			opts.Sorts = append(opts.Sorts, term)
			idx = nextIdx
		case "--limit":
			value, nextIdx, err := valueWithFallbackAllowDash(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			parsed, parseErr := parseNonNegativeInt("--limit", value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			opts.Limit = parsed
			idx = nextIdx
		case "--offset":
			value, nextIdx, err := valueWithFallbackAllowDash(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			parsed, parseErr := parseNonNegativeInt("--offset", value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			opts.Offset = parsed
			idx = nextIdx
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown query option: %s", name),
				nil,
			)
		}
	}

	return opts, nil
}

func parseSortTerm(raw string) (model.SortTerm, *domainerrors.AppError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return model.SortTerm{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--sort value cannot be empty",
			nil,
		)
	}

	path := value
	direction := model.SortDirectionAsc

	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return model.SortTerm{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("invalid --sort value: %s", raw),
				nil,
			)
		}
		path = strings.TrimSpace(parts[0])
		sortDirection := strings.TrimSpace(parts[1])
		switch sortDirection {
		case "asc":
			direction = model.SortDirectionAsc
		case "desc":
			direction = model.SortDirectionDesc
		default:
			return model.SortTerm{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("invalid sort direction in --sort: %s", raw),
				nil,
			)
		}
	}

	if path == "" {
		return model.SortTerm{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("invalid --sort field: %s", raw),
			nil,
		)
	}

	return model.SortTerm{
		Path:      path,
		Direction: direction,
	}, nil
}

func splitLongFlag(token string) (string, string, bool) {
	parts := strings.SplitN(token, "=", 2)
	if len(parts) == 1 {
		return parts[0], "", false
	}
	return parts[0], parts[1], true
}

func valueWithFallback(args []string, currentIdx int, hasInlineValue bool, inlineValue string) (string, int, *domainerrors.AppError) {
	if hasInlineValue {
		if inlineValue == "" {
			return "", currentIdx, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"option value cannot be empty",
				nil,
			)
		}
		return inlineValue, currentIdx, nil
	}

	nextIdx := currentIdx + 1
	if nextIdx >= len(args) || strings.HasPrefix(args[nextIdx], "-") {
		return "", currentIdx, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"option value is required",
			nil,
		)
	}

	return args[nextIdx], nextIdx, nil
}

func valueWithFallbackAllowDash(args []string, currentIdx int, hasInlineValue bool, inlineValue string) (string, int, *domainerrors.AppError) {
	if hasInlineValue {
		if inlineValue == "" {
			return "", currentIdx, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"option value cannot be empty",
				nil,
			)
		}
		return inlineValue, currentIdx, nil
	}

	nextIdx := currentIdx + 1
	if nextIdx >= len(args) {
		return "", currentIdx, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"option value is required",
			nil,
		)
	}

	return args[nextIdx], nextIdx, nil
}

func parseNonNegativeInt(name string, raw string) (int, *domainerrors.AppError) {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed < 0 {
		return 0, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("%s must be an integer >= 0", name),
			map[string]any{"value": raw},
		)
	}
	return parsed, nil
}
