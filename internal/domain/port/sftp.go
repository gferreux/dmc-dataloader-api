package port

import (
	"context"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

// SFTPAccounts provisions SFTPGo users for publisher and advertiser clients.
type SFTPAccounts interface {
	Config() model.SFTPConfigView
	Get(ctx context.Context, username string) (model.SFTPAccountView, error)
	Preview(ctx context.Context, req model.SFTPAccountRequest) (model.SFTPPlan, error)
	Apply(ctx context.Context, req model.SFTPAccountRequest) (model.SFTPApplyResult, error)
}
