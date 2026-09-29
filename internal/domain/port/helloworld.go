package port

import "context"

type HelloWorldUsecase interface {
	SayHello(ctx context.Context, name string) (string, error)
}
