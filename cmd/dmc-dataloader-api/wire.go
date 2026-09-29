//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/app"
	helloworldhandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/helloworld"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/usecase/helloworld"
)

// InitializeApp wires the dependency graph and builds the application.
func InitializeApp(appConfig model.AppConfig) *app.App {
	wire.Build(
		helloworld.NewHelloWorldUsecase,
		helloworldhandler.NewHandler,
		app.New,
	)

	return nil
}
