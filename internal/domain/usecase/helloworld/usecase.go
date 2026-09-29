package helloworld

import (
	"context"
	"fmt"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

type helloWorldUsecase struct{}

func NewHelloWorldUsecase() port.HelloWorldUsecase {
	return &helloWorldUsecase{}
}

func (u *helloWorldUsecase) SayHello(_ context.Context, name string) (string, error) {
	return fmt.Sprintf("Hello, %s!", name), nil
}
