package main

import (
	"github.com/innatical/apkg/cmd"
	"os"
	"os/user"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

func main() {
	usr, err := user.Current()
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	app := &cli.App{
		Name:      "apkg",
		Usage:     "The alt package manager backend",
		UsageText: "apkg [global options] command [command options] [arguments...]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "root",
				Value: filepath.Join(usr.HomeDir, "/.apkg"),
				Usage: "The root directory for the apkg package manager",
			},
		},
		Commands: []*cli.Command{
			{
				Name:      "install",
				Usage:     "Install a package",
				UsageText: "apkg install <package file>",
				Aliases:   []string{"i"},
				Action:    cmd.Install,
			},
			{
				Name:      "remove",
				Usage:     "Remove a package",
				UsageText: "apkg remove <package name>",
				Aliases:   []string{"r"},
				Action:    cmd.Remove,
			},
			{
				Name:      "list",
				Usage:     "List all installed packages",
				UsageText: "apkg list",
				Aliases:   []string{"l"},
				Action:    cmd.List,
			},
			{
				Name:      "info",
				Usage:     "Get the information for a package",
				UsageText: "apkg info <package file|package name>",
				Aliases:   []string{"in"},
				Action:    cmd.Info,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		println(err.Error())
		os.Exit(1)
	}
}
