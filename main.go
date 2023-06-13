package main

import (
	"github.com/bienkma/luks-vault/agent"
)
import "context"

func main() {
	ctx := context.Background()
	s := agent.New()
	s.Start(ctx)
}
