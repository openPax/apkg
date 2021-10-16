package cmd

import (
	"github.com/innatical/apkg/v2/util"
	"github.com/urfave/cli/v2"
)

func List(c *cli.Context) error {
	if err := util.LockDatabase(c.String("root")); err != nil {
		return err
	}

	defer util.UnlockDatabase(c.String("root"))

	installed, err := util.ListInstalled(c.String("root"))
	if err != nil {
		return err
	}

	table := make(map[string]string)
	maxWidth := 0

	for _, dbPackage := range installed {
		table[dbPackage.Package.Name+"@"+dbPackage.Package.Version] = dbPackage.Hash

		lineWidth := len(dbPackage.Package.Name) + 1 + len(dbPackage.Package.Version) + 5 + len(dbPackage.Hash)
		if lineWidth > maxWidth {
			maxWidth = lineWidth
		}
	}

	println(util.RenderTable(table, maxWidth))

	return nil
}
