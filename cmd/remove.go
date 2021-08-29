package cmd

import (
	"github.com/innatical/apkg/util"
	"github.com/urfave/cli/v2"
)

func Remove(c *cli.Context) error {
	if err := util.Remove(c.String("root"), c.Args().First()); err != nil {
		return err
	}

	return nil
}
