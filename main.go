package main

import (
	"os"

	"github.com/BlasterM2A/shell-manager/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
