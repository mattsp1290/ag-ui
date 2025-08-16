package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/mattsp1290/ag-ui/go-cli/cmd/ag-ui-cli/commands"
)

func main() {
	// Use Fang to execute the root command with enhanced features
	if err := fang.Execute(context.Background(), commands.RootCmd); err != nil {
		os.Exit(1)
	}
}