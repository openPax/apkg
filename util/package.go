package util

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/ulikunitz/xz"
)

type PackageRoot struct {
	Spec			int `toml:"spec"`
	Package			Package  `toml:"package"`
	Dependencies	Dependencies `toml:"dependencies"`
	Bin 			map[string]string `toml:"bin"`
	Hooks			Hooks `toml:"hooks"`
}

type Package struct {
	Name			string `toml:"name"`
	Description		string `toml:"description"`
	Version			string `toml:"version"`
	Authors			[]string `toml:"authors"`
	Maintainers		[]string `toml:"maintainers"`
}

type Dependencies struct {
	Required	[]string `toml:"required"`
	Optional	[]string `toml:"optional"`
}

type Hooks struct {
	Postinstall	string `toml:"postinstall"`
	Preinstall	string `toml:"preinstall"`
	Postremove 	string `toml:"postremove"`
	Preremove	string `toml:"preremove"`
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