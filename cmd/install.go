package cmd

import (
	"github.com/innatical/apkg/util"

	"github.com/urfave/cli/v2"
)

func Install(c *cli.Context) error {
	if err := util.Install(c.String("root"), c.Args().First()); err != nil {
		return err
	}

	return nil
}