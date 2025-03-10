package main

import "github.com/urfave/cli/v2"

var configCmd = &cli.Command{
	Name:    "config",
	Aliases: []string{"cfg"},
	Usage:   "Manage 3fs config",
	Subcommands: []*cli.Command{
		{
			Name:  "create",
			Usage: "Create a sample 3fs config",
		},
	},
}
