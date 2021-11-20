package cmd

import (
	"github.com/innatical/apkg/util"
	"github.com/urfave/cli/v2"
)

func Verify(c *cli.Context) error {

	if err := util.LockDatabase(c.String("root")); err != nil {
		return err
	}

	defer util.UnlockDatabase(c.String("root"))

	checksum, err := util.ValidateChecksum(c.Args().First(), c.Args().Get(1))

	if err != nil {
		return err
	} else if checksum {
		println("Checksums Matched")
	} else {
		println("Checksums did not match")
	}

	return nil
}
