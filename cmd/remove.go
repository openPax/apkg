package cmd

import (
	"apkg/util"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

func Remove (c *cli.Context) error {
	if err := os.MkdirAll(c.String("root"), 0755); err != nil {
		return err
	}

	if err := util.LockDatabase(c.String("root")); err != nil {
		return err
	}

	defer util.UnlockDatabase(c.String("root"))

	packageName := c.Args().First()

	db, err := util.ReadDatabase(c.String("root"))

	if err != nil {
		return err
	}

	if _, ok := db.Packages[packageName]; !ok {
		return &errorString{"Package doesn't exist"}
	}

	for name, pkg := range db.Packages {
		for i := range pkg.Dependencies.Required {
			if strings.Split(pkg.Dependencies.Required[i], "@")[0] == packageName {
				return &errorString{ "Package " + name + " depends on " + packageName}
			}
		}
	}

	installationPath := filepath.Join(c.String("root"), "packages", db.Packages[packageName].Hash)

	pkg, err := util.ParsePackageFile(filepath.Join(installationPath, "package.toml"))

	if err != nil {
  	return err
	}

	if pkg.Hooks.Preremove != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Preremove), 0755); err != nil {
			return err
		}
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Preremove))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			return err
		}
	}

	util.RemoveBinaries(c.String("root"), pkg)
	
	if pkg.Hooks.Postremove != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Postremove), 0755); err != nil {
			return err
		}
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Postremove))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(installationPath); err != nil {
		return err
	}

	delete(db.Packages, packageName)

	util.WriteDatabase(c.String("root"), db)

	return nil
}