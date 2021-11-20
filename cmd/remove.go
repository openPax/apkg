package cmd

import (
	"github.com/innatical/apkg/v2/util"
	"github.com/urfave/cli/v2"
)

func Remove(c *cli.Context) error {
	removal_flag := c.Bool("remove-core-this-will-probably-break-my-system")

	if err := util.LockDatabase(c.String("root")); err != nil {
		return err
	}

	defer util.UnlockDatabase(c.String("root"))

	if err := util.Remove(c.String("root"), c.Args().First(), removal_flag); err != nil {
		return err
	}

	return nil
}
