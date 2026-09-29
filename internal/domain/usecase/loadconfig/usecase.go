// Package loadconfig implements load_config validation and persistence rules.
package loadconfig

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/catalog"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

const (
	maxIDLen      = 200
	maxPatternLen = 1024
)

var errPatternTooLong = errors.New("pattern is too long")

type usecase struct {
	repo      port.LoadConfigRepository
	directory port.OrganizationDirectory
	settings  model.DeriveConfig
	logger    *slog.Logger
}

// ProvideLogger is the process logger used for audit lines.
func ProvideLogger() *slog.Logger {
	return slog.Default()
}

// ProvideSettings selects the plumbing used to derive a load_config.
func ProvideSettings(cfg model.AppConfig) model.DeriveConfig {
	return cfg.Derive
}

// NewUsecase builds the load_config usecase.
func NewUsecase(
	repo port.LoadConfigRepository,
	directory port.OrganizationDirectory,
	settings model.DeriveConfig,
	logger *slog.Logger,
) port.LoadConfigUsecase {
	if logger == nil {
		logger = slog.Default()
	}

	return &usecase{repo: repo, directory: directory, settings: settings, logger: logger}
}

func (u *usecase) List(ctx context.Context, filter model.ListFilter) ([]model.LoadConfig, error) {
	configs, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	filter.PartnerType = strings.ToLower(strings.TrimSpace(filter.PartnerType))
	filter.ImportType = strings.ToLower(strings.TrimSpace(filter.ImportType))
	filter.Query = strings.TrimSpace(filter.Query)

	items := make([]model.LoadConfig, 0, len(configs))
	for _, cfg := range configs {
		presented := cfg.Present()
		if filter.PartnerType != "" && presented.PartnerType != filter.PartnerType {
			continue
		}

		if filter.ImportType != "" && presented.ImportType != filter.ImportType {
			continue
		}

		if !matchesQuery(presented, filter.Query) {
			continue
		}

		items = append(items, presented)
	}

	sortByID(items)

	return items, nil
}

func (u *usecase) Get(ctx context.Context, id string) (model.LoadConfig, error) {
	cfg, err := u.repo.Get(ctx, id)
	if err != nil {
		return model.LoadConfig{}, err
	}

	return cfg.Present(), nil
}

func (u *usecase) Create(ctx context.Context, cfg model.LoadConfig) (model.LoadConfig, error) {
	cfg = model.Normalize(cfg)

	derived, _, err := u.materialize(ctx, cfg.Identity)
	if err != nil {
		return model.LoadConfig{}, err
	}

	derived.Mode = cfg.Mode
	derived.BQParams = cfg.BQParams

	derived.Mappings = cfg.Mappings
	if err = u.rejectInvalid(derived); err != nil {
		return model.LoadConfig{}, err
	}

	created, err := u.repo.Create(ctx, derived)
	if err != nil {
		return model.LoadConfig{}, err
	}

	u.audit(ctx, "created", created.ID)

	return created.Present(), nil
}

func (u *usecase) Update(ctx context.Context, id string, cfg model.LoadConfig) (model.LoadConfig, error) {
	cfg = model.Normalize(cfg)

	existing, err := u.repo.Get(ctx, id)
	if err != nil {
		return model.LoadConfig{}, err
	}

	merged, err := u.mergeUpdate(ctx, id, existing, cfg)
	if err != nil {
		return model.LoadConfig{}, err
	}

	if err = u.rejectInvalid(merged); err != nil {
		return model.LoadConfig{}, err
	}

	updated, err := u.repo.Move(ctx, id, merged)
	if err != nil {
		return model.LoadConfig{}, err
	}

	u.audit(ctx, "updated", updated.ID)

	return updated.Present(), nil
}

func (u *usecase) Delete(ctx context.Context, id string) error {
	if err := u.repo.Delete(ctx, id); err != nil {
		return err
	}

	u.audit(ctx, "deleted", id)

	return nil
}

func (u *usecase) audit(ctx context.Context, action, id string) {
	u.logger.Info(
		"load config "+action,
		"action", action,
		"id", id,
		"email", port.PrincipalFrom(ctx).Email,
	)
}

func (u *usecase) Validate(ctx context.Context, cfg model.LoadConfig) (model.ValidationReport, error) {
	cfg = model.Normalize(cfg)

	others, err := u.repo.List(ctx)
	if err != nil {
		return model.ValidationReport{}, err
	}

	return review(cfg, others), nil
}

func (u *usecase) TestPattern(ctx context.Context, pattern, path string) (model.PatternTest, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return model.PatternTest{}, &model.ValidationError{Issues: []model.FieldIssue{{
			Field:   "pattern",
			Message: "pattern is required",
		}}}
	}

	compiled, err := compilePattern(pattern)
	if err != nil {
		return model.PatternTest{}, &model.ValidationError{Issues: []model.FieldIssue{{
			Field:   "pattern",
			Message: "pattern is not a valid regex: " + err.Error(),
		}}}
	}

	configs, err := u.repo.List(ctx)
	if err != nil {
		return model.PatternTest{}, err
	}

	sortByID(configs)

	result := model.PatternTest{Matches: compiled.MatchString(path)}

	for _, cfg := range configs {
		if configMatches(cfg, path) {
			id := cfg.ID
			result.MatchingConfigID = &id

			break
		}
	}

	return result, nil
}

func (u *usecase) Templates(context.Context) []model.Template {
	return catalog.Templates()
}

func (u *usecase) Meta(context.Context) model.Meta {
	return catalog.Meta()
}

func (u *usecase) rejectInvalid(cfg model.LoadConfig) error {
	report := review(cfg, nil)
	if len(report.Errors) == 0 {
		return nil
	}

	return &model.ValidationError{Issues: report.Errors}
}

func sortByID(configs []model.LoadConfig) {
	slices.SortFunc(configs, func(left, right model.LoadConfig) int {
		return strings.Compare(left.ID, right.ID)
	})
}

func matchesQuery(cfg model.LoadConfig, query string) bool {
	if query == "" {
		return true
	}

	needle := strings.ToLower(query)

	haystack := []string{
		cfg.ID,
		cfg.PublisherName,
		cfg.PartnerType,
		cfg.ImportType,
		cfg.Destination.ProjectID,
		cfg.Destination.DatasetID,
		cfg.Destination.TableID,
		cfg.Patterns.Preprocess,
		cfg.Patterns.Ingest,
		cfg.Kind,
		cfg.OrganizationName,
		cfg.NestedName,
		cfg.FileType,
		cfg.Organization.ID,
		string(cfg.Organization.Type),
	}
	if cfg.Organization.Account != nil {
		haystack = append(haystack, *cfg.Organization.Account)
	}

	for _, value := range haystack {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}

	return false
}

func configMatches(cfg model.LoadConfig, path string) bool {
	for _, pattern := range []string{cfg.Patterns.Preprocess, cfg.Patterns.Ingest} {
		if pattern == "" {
			continue
		}

		compiled, err := compilePattern(pattern)
		if err != nil {
			continue
		}

		if compiled.MatchString(path) {
			return true
		}
	}

	return false
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > maxPatternLen {
		return nil, errPatternTooLong
	}

	return regexp.Compile(pattern)
}
