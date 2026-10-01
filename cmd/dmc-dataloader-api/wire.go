//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/auth"
	firestoreadapter "github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/firestore"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/sftpgo"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/app"
	loadconfighandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/loadconfig"
	sftphandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/sftpaccount"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/usecase/loadconfig"
)

// InitializeApp wires the dependency graph and builds the application.
func InitializeApp(ctx context.Context, appConfig model.AppConfig) (*app.App, func(), error) {
	wire.Build(
		firestoreadapter.NewRepository,
		wire.Bind(new(port.LoadConfigRepository), new(*firestoreadapter.Repository)),
		firestoreadapter.NewDirectory,
		wire.Bind(new(port.OrganizationDirectory), new(*firestoreadapter.Directory)),
		auth.NewAuthenticator,
		loadconfig.ProvideLogger,
		loadconfig.ProvideSettings,
		loadconfig.NewUsecase,
		loadconfighandler.NewHandler,
		sftpgo.ProvideService,
		wire.Bind(new(port.SFTPAccounts), new(*sftpgo.Service)),
		sftphandler.NewHandler,
		app.New,
	)

	return nil, nil, nil
}
