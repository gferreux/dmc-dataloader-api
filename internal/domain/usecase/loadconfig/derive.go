package loadconfig

import (
	"context"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func (u *usecase) Derive(ctx context.Context, identity model.Identity) (model.DerivedConfig, error) {
	built, warning, err := u.materialize(ctx, identity)
	if err != nil {
		return model.DerivedConfig{}, err
	}

	others, err := u.repo.List(ctx)
	if err != nil {
		return model.DerivedConfig{}, err
	}

	warnings := collectWarnings(built, others)
	if warning != nil {
		warnings = append(warnings, *warning)
	}

	return model.DerivedConfig{
		ID:            built.ID,
		PublisherName: built.PublisherName,
		Patterns:      built.Patterns,
		Notification:  built.Notification,
		Destination:   built.Destination,
		Organization:  built.Organization,
		Warnings:      warnings,
	}, nil
}

func (u *usecase) materialize(
	ctx context.Context,
	identity model.Identity,
) (model.LoadConfig, *model.FieldIssue, error) {
	identity = identity.Normalized()

	built, err := model.BuildDerived(identity, u.settings)
	if err != nil {
		return model.LoadConfig{}, nil, err
	}

	org, warning, err := u.resolve(ctx, identity)
	if err != nil {
		return model.LoadConfig{}, nil, err
	}

	built.Organization = org

	return built, warning, nil
}

func (u *usecase) mergeUpdate(
	ctx context.Context,
	id string,
	existing model.LoadConfig,
	incoming model.LoadConfig,
) (model.LoadConfig, error) {
	merged := existing
	merged.ID = id
	merged.Mode = incoming.Mode
	merged.BQParams = incoming.BQParams
	merged.Mappings = incoming.Mappings
	merged.PartnerType = ""
	merged.ImportType = ""
	merged.Identity = model.Identity{}

	incomingID := incoming.Normalized()
	if !incomingID.Provided() {
		return merged, nil
	}

	if parsed, ok := model.ParseIdentity(existing); ok && model.SameIdentity(parsed, incomingID) {
		return merged, nil
	}

	derived, _, err := u.materialize(ctx, incomingID)
	if err != nil {
		return model.LoadConfig{}, err
	}

	merged.ID = derived.ID
	merged.PublisherName = derived.PublisherName
	merged.Patterns = derived.Patterns
	merged.Destination = derived.Destination
	merged.Notification = derived.Notification
	merged.Organization = derived.Organization

	return merged, nil
}

func collectWarnings(cfg model.LoadConfig, others []model.LoadConfig) []model.FieldIssue {
	check := &checker{report: model.EmptyReport()}
	check.warnings(cfg, others)

	for _, other := range others {
		if other.ID == cfg.ID {
			check.warn("id", "a load config with this id already exists")

			break
		}
	}

	if check.report.Warnings == nil {
		return []model.FieldIssue{}
	}

	return check.report.Warnings
}
