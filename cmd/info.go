package cmd

import (
	"github.com/innatical/apkg/util"
	"github.com/urfave/cli/v2"
	"os"
)

func Info(c *cli.Context) error {
	var pkg *util.PackageRoot

	_, err := os.Stat(c.Args().First())
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		} else {
			pkg, err = util.PackageInfo(c.String("root"), c.Args().First())
			if err != nil {
				return err
			}
		}
	} else {
		pkg, err = util.InspectPackage(c.Args().First())
		if err != nil {
			return err
		}
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
