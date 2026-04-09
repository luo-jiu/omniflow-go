package main

import (
	"os"

	transportcli "omniflow-go/internal/transport/cli"
)

func main() {
	os.Exit(transportcli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
