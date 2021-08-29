package cmd

import (
	"github.com/innatical/apkg/util"
	"github.com/urfave/cli/v2"
)

func Info(c *cli.Context) error {
	pkg, err := util.InspectPackage(c.Args().First())
	if err != nil {
		return err
	}

	println(pkg.Package.Name + "@" + pkg.Package.Version)
	println(pkg.Package.Description)

	println()

	println("Authors:")
	for i := range pkg.Package.Authors {
		println(pkg.Package.Authors[i])
	}

	println()

	println("Maintainers:")
	for i := range pkg.Package.Maintainers {
		println(pkg.Package.Maintainers[i])
	}

	println()

	println("Dependencies:")
	for i := range pkg.Dependencies.Required {
		println(pkg.Dependencies.Required[i])
	}

	println()

	println("Optional Dependencies:")
	for i := range pkg.Dependencies.Optional {
		println(pkg.Dependencies.Optional[i])
	}

	return nil
}
