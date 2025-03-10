package main

import "github.com/urfave/cli/v2"

var tmplCmd = &cli.Command{
	Name:    "template",
	Aliases: []string{"t"},
	Usage:   "Service config template operate",
	Subcommands: []*cli.Command{
		{
			Name:  "create",
			Usage: "Create 3fs service config template",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "service",
					Usage:    "service name",
					Required: true,
				},
			},
		},
	},
}
