package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "m3fs",
		Usage: "3FS Deploy Tool",
		Commands: []*cli.Command{
			clusterCmd,
			configCmd,
			osCmd,
			tmplCmd,
		},
		Action: func(ctx *cli.Context) error {
			return cli.ShowAppHelp(ctx)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
