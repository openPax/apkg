package main

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/innatical/apkg/v2/cmd"

	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
)

var errorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF0000"))

func main() {
	usr, err := user.Current()
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	app := &cli.App{
		Name:      "apkg",
		Usage:     "The pax package manager backend",
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
				UsageText: "apkg install <package files...>",
				Aliases:   []string{"i"},
				Action:    cmd.Install,
			},
			{
				Name:      "remove",
				Usage:     "Remove a package",
				UsageText: "apkg remove <package name>",
				Aliases:   []string{"r"},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "remove-core-this-will-probably-break-my-system",
						Usage: "Allows removal of Core packages",
					},
				},
				Action: cmd.Remove,
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
		println(errorStyle.Render("Error: ") + err.Error())
		os.Exit(1)
	}
}
