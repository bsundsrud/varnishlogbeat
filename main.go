package main

import (
	"os"

	"github.com/bsundsrud/varnishlogbeat/cmd"

	_ "github.com/bsundsrud/varnishlogbeat/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
