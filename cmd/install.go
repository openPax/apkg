package cmd

import (
	"github.com/innatical/apkg/v2/util"

	"github.com/urfave/cli/v2"
)

func Install(c *cli.Context) error {
	if err := util.LockDatabase(c.String("root")); err != nil {
		return err
	}

	defer util.UnlockDatabase(c.String("root"))

	if err := util.InstallMultiple(c.String("root"), c.Args().Slice()); err != nil {
		return err
	}

	return nil
}
