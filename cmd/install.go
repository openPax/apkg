package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"apkg/util"

	"github.com/Masterminds/semver"
	"github.com/urfave/cli/v2"
)

func Install (c *cli.Context) error {
	if err := os.MkdirAll(c.String("root"), 0755); err != nil {
		return err
	}

	if err := util.LockDatabase(c.String("root")); err != nil {
		return err
	}

	defer util.UnlockDatabase(c.String("root"))

	packageFile := c.Args().First()

	file, err := os.Open(packageFile)
	if err != nil {
		return &errorString{"Couldn't open file!"}
	}
	defer file.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
    return err
	}

	stringHash := hex.EncodeToString(hasher.Sum(nil))
	installationPath := filepath.Join(c.String("root"), "packages", stringHash)

	if err := os.MkdirAll(installationPath, 0755); err != nil {
		return err
	}

	if err := util.ExtractPackage(packageFile, installationPath); err != nil {
		return err
	}

	pkg, err := util.ParsePackageFile(filepath.Join(installationPath, "package.toml"))

	if err != nil {
    return err
	}

	db, err := util.ReadDatabase(c.String("root"))

	if err != nil {
		return err
	}

	if _, ok := db.Packages[pkg.Package.Name]; ok {
		return &errorString{"Package is already installed with name " + pkg.Package.Name}
	}

	for i := range pkg.Dependencies.Required {
		dependency := pkg.Dependencies.Required[i]
		splitdep := strings.Split(dependency, "@")

		if _, ok := db.Packages[splitdep[0]]; !ok {
			return &errorString{"Dependency not found: " + dependency}
		}

		depVersion, err := semver.NewVersion(db.Packages[splitdep[0]].Package.Version)
		if err != nil {
			return err
		}

		c, err := semver.NewConstraint(splitdep[1])
		if err != nil {
			return err
		}

		if !c.Check(depVersion) {
			return &errorString{"Version constraint for package " + splitdep[0] + "not met. Required " + splitdep[1] + ", Found " + db.Packages[splitdep[0]].Package.Version}
		}
	}

	if pkg.Hooks.Preinstall != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Preinstall), 0755); err != nil {
			return err
		}
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Preinstall))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			return err
		}
	}

	if err := util.InstallBinaries(c.String("root"), installationPath, pkg); err != nil {
		return err
	}

	db.Packages[pkg.Package.Name] = util.DBPackage{Hash: stringHash, Dependencies: pkg.Dependencies, Package: pkg.Package}

	if err := util.WriteDatabase(c.String("root"), db); err != nil {
		return err
	}

	if pkg.Hooks.Postinstall != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Postinstall), 0755); err != nil {
			return err
		}
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Postinstall))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}