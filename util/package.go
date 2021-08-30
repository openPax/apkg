package util

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"github.com/Masterminds/semver"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ulikunitz/xz"
)

type PackageRoot struct {
	Spec         int               `toml:"spec"`
	Package      Package           `toml:"package"`
	Dependencies Dependencies      `toml:"dependencies"`
	Bin          map[string]string `toml:"bin"`
	Lib          map[string]string `toml:"lib"`
	Hooks        Hooks             `toml:"hooks"`
}

type Package struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Version     string   `toml:"version"`
	Authors     []string `toml:"authors"`
	Maintainers []string `toml:"maintainers"`
}

type Dependencies struct {
	Required []string `toml:"required"`
	Optional []string `toml:"optional"`
}

type Hooks struct {
	Postinstall string `toml:"postinstall"`
	Preinstall  string `toml:"preinstall"`
	Postremove  string `toml:"postremove"`
	Preremove   string `toml:"preremove"`
}

func ParsePackageFile(path string) (*PackageRoot, error) {
	var pkg PackageRoot

	if _, err := toml.DecodeFile(path, &pkg); err != nil {
		return nil, err
	}

	return &pkg, nil
}

func ExtractPackage(tarball, target string) error {
	reader, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer reader.Close()
	xzReader, err := xz.NewReader(reader)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(xzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(target, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, tarReader)
		if err != nil {
			return err
		}
	}
	return nil
}

func InspectPackage(tarball string) (*PackageRoot, error) {
	reader, err := os.Open(tarball)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	xzReader, err := xz.NewReader(reader)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(xzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		if header.Name == "package.toml" {
			var pkg PackageRoot

			if _, err := toml.DecodeReader(tarReader, &pkg); err != nil {
				return nil, err
			}

			return &pkg, nil
		}
	}

	return nil, &ErrorString{S: "package.toml not found"}
}

func InstallBinaries(root string, pkgPath string, pkg *PackageRoot) error {
	for k, v := range pkg.Bin {
		if err := os.Chmod(filepath.Join(pkgPath, v), 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(root, "bin"), 0755); err != nil {
			return err
		}
		if err := os.Symlink(filepath.Join(pkgPath, v), filepath.Join(root, "bin", k)); err != nil {
			return err
		}
	}

	return nil
}

func RemoveBinaries(root string, pkg *PackageRoot) error {
	for k := range pkg.Bin {
		if err := os.Remove(filepath.Join(root, "bin", k)); err != nil {
			return err
		}
	}

	return nil
}

func InstallLibraries(root string, pkgPath string, pkg *PackageRoot) error {
	for k, v := range pkg.Lib {
		if err := os.Chmod(filepath.Join(pkgPath, v), 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(root, "lib"), 0755); err != nil {
			return err
		}
		if err := os.Symlink(filepath.Join(pkgPath, v), filepath.Join(root, "lib", k)); err != nil {
			return err
		}
	}

	return nil
}

func RemoveLibraries(root string, pkg *PackageRoot) error {
	for k := range pkg.Lib {
		if err := os.Remove(filepath.Join(root, "lib", k)); err != nil {
			return err
		}
	}

	return nil
}

func Install(root string, packageFile string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	if err := LockDatabase(root); err != nil {
		return err
	}

	defer UnlockDatabase(root)

	db, err := ReadDatabase(root)

	if err != nil {
		return err
	}

	pkg, err := InspectPackage(packageFile)

	if err != nil {
		return err
	}

	if _, ok := db.Packages[pkg.Package.Name]; ok {
		return &ErrorString{S: "Package is already installed with name " + pkg.Package.Name}
	}

	file, err := os.Open(packageFile)
	if err != nil {
		return &ErrorString{S: "Couldn't open file!"}
	}
	defer file.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	stringHash := hex.EncodeToString(hasher.Sum(nil))
	installationPath := filepath.Join(root, "packages", stringHash)

	if err := os.MkdirAll(installationPath, 0755); err != nil {
		return err
	}

	if err := ExtractPackage(packageFile, installationPath); err != nil {
		return err
	}

	if err != nil {
		return err
	}

	for i := range pkg.Dependencies.Required {
		dependency := pkg.Dependencies.Required[i]
		splitdep := strings.Split(dependency, "@")

		if _, ok := db.Packages[splitdep[0]]; !ok {
			return &ErrorString{S: "Dependency not found: " + dependency}
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
			return &ErrorString{S: "Version constraint for package " + splitdep[0] + "not met. Required " + splitdep[1] + ", Found " + db.Packages[splitdep[0]].Package.Version}
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

	if err := InstallBinaries(root, installationPath, pkg); err != nil {
		return err
	}
	if err := InstallLibraries(root, installationPath, pkg); err != nil {
		return err
	}

	db.Packages[pkg.Package.Name] = DBPackage{Hash: stringHash, Dependencies: pkg.Dependencies, Package: pkg.Package}

	if err := WriteDatabase(root, db); err != nil {
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

func Remove(root string, packageName string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	if err := LockDatabase(root); err != nil {
		return err
	}

	defer UnlockDatabase(root)

	db, err := ReadDatabase(root)

	if err != nil {
		return err
	}

	if _, ok := db.Packages[packageName]; !ok {
		return &ErrorString{S: "Package doesn't exist"}
	}

	for name, pkg := range db.Packages {
		for i := range pkg.Dependencies.Required {
			if strings.Split(pkg.Dependencies.Required[i], "@")[0] == packageName {
				return &ErrorString{S: "Package " + name + " depends on " + packageName}
			}
		}
	}

	installationPath := filepath.Join(root, "packages", db.Packages[packageName].Hash)

	pkg, err := ParsePackageFile(filepath.Join(installationPath, "package.toml"))

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

	if err := RemoveBinaries(root, pkg); err != nil {
		return err
	}
	if err := RemoveLibraries(root, pkg); err != nil {
		return err
	}

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

	if err := WriteDatabase(root, db); err != nil {
		return err
	}

	return nil
}

func ListInstalled(root string) (map[string]DBPackage, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}

	if err := LockDatabase(root); err != nil {
		return nil, err
	}

	defer UnlockDatabase(root)

	db, err := ReadDatabase(root)

	if err != nil {
		return nil, err
	}

	return db.Packages, nil
}

func PackageInfo(root string, name string) (pkg *PackageRoot, err error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}

	if err := LockDatabase(root); err != nil {
		return nil, err
	}

	defer UnlockDatabase(root)

	db, err := ReadDatabase(root)

	if err != nil {
		return nil, err
	}

	if _, ok := db.Packages[name]; !ok {
		return nil, &ErrorString{S: "Package doesn't exist"}
	}

	installationPath := filepath.Join(root, "packages", db.Packages[name].Hash)

	pkg, err = ParsePackageFile(filepath.Join(installationPath, "package.toml"))

	if err != nil {
		return nil, err
	}

	return pkg, nil
}
