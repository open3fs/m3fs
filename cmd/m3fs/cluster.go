package main

import "github.com/urfave/cli/v2"

var clusterCmd = &cli.Command{
	Name:    "cluster",
	Aliases: []string{"c"},
	Usage:   "Manage 3fs cluster",
	Subcommands: []*cli.Command{
		{
			Name:  "create",
			Usage: "Create a new 3fs cluster",
		},
	},
}
