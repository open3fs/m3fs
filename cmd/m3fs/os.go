package main

import "github.com/urfave/cli/v2"

var osCmd = &cli.Command{
	Name:  "os",
	Usage: "Manage os environment",
	Subcommands: []*cli.Command{
		{
			Name:  "init",
			Usage: "Initialize os environment",
		},
	},
}
