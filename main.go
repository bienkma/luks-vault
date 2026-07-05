package main

import (
	"context"

	"github.com/bienkma/luks-vault/agent"
)

func main() {
	ctx := context.Background()
	s := agent.New()
	s.Start(ctx)
}
