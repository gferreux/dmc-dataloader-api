package helloworld_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/usecase/helloworld"
)

func TestSayHello(t *testing.T) {
	t.Parallel()

	uc := helloworld.NewHelloWorldUsecase()

	msg, err := uc.SayHello(context.Background(), "World")

	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", msg)
}
