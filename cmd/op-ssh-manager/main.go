package main

import (
	"os"

	"github.com/kerren/op-ssh-manager/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
