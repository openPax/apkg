package util

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/goombaio/dag"
	"golang.org/x/sync/errgroup"

	"github.com/BurntSushi/toml"
	"github.com/klauspost/compress/zstd"
)

type PackageRoot struct {
	Spec         int               `toml:"spec"`
	Package      Package           `toml:"package"`
	Dependencies Dependencies      `toml:"dependencies"`
	Files        map[string]string `toml:"files"`
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

var dbLock sync.Mutex

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
	zstdReader, err := zstd.NewReader(reader)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(zstdReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(target, header.Name)
		info := header.FileInfo()

		if header.Typeflag == tar.TypeLink {
			if err := os.Link(filepath.Join(target, header.Linkname), path); err != nil {
				return err
			}
			continue
		}

		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}

		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			if err = os.Symlink(header.Linkname, path); err != nil {
				return err
			}
			continue
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(file, tarReader)
		if err != nil {
			file.Close()
			return err
		}
		file.Close()
	}
	return nil
}

func InspectPackage(tarball string) (*PackageRoot, error) {
	reader, err := os.Open(tarball)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	zstdReader, err := zstd.NewReader(reader)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(zstdReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		if path.Clean(header.Name) == "package.toml" {
			var pkg PackageRoot

			if _, err := toml.DecodeReader(tarReader, &pkg); err != nil {
				return nil, err
			}

			return &pkg, nil
		}
	}

	return nil, &ErrorString{S: "package.toml not found"}
}

var globalSideEffectLock sync.Mutex

func InstallFiles(root string, pkgPath string, pkg *PackageRoot) error {
	for k, v := range pkg.Files {
		info, err := os.Stat(filepath.Join(pkgPath, v))
		if err != nil {
			return err
		}

		if info.IsDir() {
			if err := filepath.Walk(filepath.Join(pkgPath, v), func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				relative, err := filepath.Rel(filepath.Join(pkgPath, v), path)
				if err != nil {
					return err
				}

				if info.IsDir() {
					os.Mkdir(filepath.Join(root, k, relative), info.Mode().Perm())
				} else {
					info, err := os.Stat(filepath.Dir(filepath.Join(pkgPath, v, relative)))
					if err != nil {
						return err
					}

					if err := os.MkdirAll(filepath.Dir(filepath.Join(root, k, relative)), info.Mode().Perm()); err != nil {
						return err
					}

					if err := os.Link(filepath.Join(pkgPath, v, relative), filepath.Join(root, k, relative)); err != nil {
						return err
					}
				}

				return nil
			}); err != nil {
				return err
			}
		} else {
			info, err := os.Stat(filepath.Dir(filepath.Join(pkgPath, v)))
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, k)), info.Mode().Perm()); err != nil {
				return err
			}

			if err := os.Link(filepath.Join(pkgPath, v), filepath.Join(root, k)); err != nil {
				return err
			}
		}
	}

	return nil
}

func RemoveFiles(root string, pkgPath string, pkg *PackageRoot) error {
	for k, v := range pkg.Files {
		info, err := os.Stat(filepath.Join(pkgPath, v))
		if err != nil {
			return err
		}

		if info.IsDir() {
			if err := filepath.Walk(filepath.Join(pkgPath, v), func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				relative, err := filepath.Rel(pkgPath, path)
				if err != nil {
					return err
				}

				if !info.IsDir() {
					if err := os.Remove(filepath.Join(root, relative)); err != nil {
						return err
					}
				}

				return nil
			}); err != nil {
				return nil
			}
		} else {
			if err := os.Remove(filepath.Join(root, k)); err != nil {
				return err
			}
		}
	}

	return nil
}

func Install(root string, packageFile string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	pkg, err := InspectPackage(packageFile)

	if err != nil {
		return err
	}

	if err := func() error {
		dbLock.Lock()
		defer dbLock.Unlock()

		db, err := ReadDatabase(root)

		if err != nil {
			return err
		}

		if _, ok := db.Packages[pkg.Package.Name]; ok {
			return &ErrorString{S: "Package is already installed with name " + pkg.Package.Name}
		}

		return nil
	}(); err != nil {
		return err
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

	if err := func() error {
		dbLock.Lock()
		defer dbLock.Unlock()

		db, err := ReadDatabase(root)

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
				return &ErrorString{S: "Version constraint for package " + splitdep[0] + "not met. Required " + splitdep[1] + ", found " + db.Packages[splitdep[0]].Package.Version}
			}
		}

		return nil
	}(); err != nil {
		return err
	}

	if pkg.Hooks.Preinstall != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Preinstall), 0755); err != nil {
			return err
		}
		globalSideEffectLock.Lock()
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Preinstall))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			globalSideEffectLock.Unlock()
			return err
		}

		globalSideEffectLock.Unlock()
	}

	if err := InstallFiles(root, installationPath, pkg); err != nil {
		return err
	}

	dbLock.Lock()

	db, err := ReadDatabase(root)

	if err != nil {
		return err
	}

	db.Packages[pkg.Package.Name] = DBPackage{Hash: stringHash, Dependencies: pkg.Dependencies, Package: pkg.Package}

	if err := WriteDatabase(root, db); err != nil {
		dbLock.Unlock()
		return err
	}

	dbLock.Unlock()

	if pkg.Hooks.Postinstall != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Postinstall), 0755); err != nil {
			return err
		}
		globalSideEffectLock.Lock()
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Postinstall))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			globalSideEffectLock.Unlock()
			return err
		}
		globalSideEffectLock.Unlock()
	}

	return nil
}

func WorkerReady(point *dag.Vertex, state map[string]string) bool {
	for _, dep := range point.Children.Values() {
		if op, ok := state[dep.(*dag.Vertex).ID]; !ok || op != "done" {
			return false
		}
	}

	return true
}

func InstallWorker(root string, point *dag.Vertex, group *errgroup.Group, state map[string]string, stateLock *sync.Mutex, completedEvent *sync.Cond) {
	group.Go(func() error {
		stateLock.Lock()
		_, ok := state[point.ID]
		if ok {
			stateLock.Unlock()
			return nil
		}

		state[point.ID] = "working"
		stateLock.Unlock()

		for {
			stateLock.Lock()
			ready := WorkerReady(point, state)
			stateLock.Unlock()

			if ready {
				break
			}

			completedEvent.Wait()
		}

		if err := Install(root, point.ID); err != nil {
			return err
		}

		stateLock.Lock()
		state[point.ID] = "done"
		stateLock.Unlock()

		completedEvent.Broadcast()

		for _, child := range point.Parents.Values() {
			InstallWorker(root, child.(*dag.Vertex), group, state, stateLock, completedEvent)
		}

		return nil
	})
}

func InstallMultiple(root string, packageFiles []string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	packages := dag.NewDAG()

	if err := func() error {
		dbLock.Lock()
		defer dbLock.Unlock()
		db, err := ReadDatabase(root)

		if err != nil {
			return err
		}

		for _, file := range packageFiles {
			pkg, err := InspectPackage(file)

			if err != nil {
				return err
			}

			packages.AddVertex(dag.NewVertex(file, pkg))
		}

		for _, file := range packageFiles {
			vertex, err := packages.GetVertex(file)

			if err != nil {
				return err
			}

			pkg := vertex.Value.(*PackageRoot)

		REQUIRED:
			for i := range pkg.Dependencies.Required {
				dependency := pkg.Dependencies.Required[i]
				splitdep := strings.Split(dependency, "@")

				if _, ok := db.Packages[splitdep[0]]; ok {
					depVersion, err := semver.NewVersion(db.Packages[splitdep[0]].Package.Version)
					if err != nil {
						return err
					}

					c, err := semver.NewConstraint(splitdep[1])
					if err != nil {
						return err
					}

					if !c.Check(depVersion) {
						return &ErrorString{S: "Version constraint for package " + splitdep[0] + "not met. Required " + splitdep[1] + ", found " + db.Packages[splitdep[0]].Package.Version}
					}

					continue
				}

				for _, file2 := range packageFiles {
					vertex2, err := packages.GetVertex(file2)

					if err != nil {
						return err
					}

					pkg2 := vertex2.Value.(*PackageRoot)

					if pkg2.Package.Name != splitdep[0] {
						continue
					}

					depVersion, err := semver.NewVersion(pkg2.Package.Version)
					if err != nil {
						return err
					}

					c, err := semver.NewConstraint(splitdep[1])
					if err != nil {
						return err
					}

					if !c.Check(depVersion) {
						return &ErrorString{S: "Version constraint for package " + splitdep[0] + "not met. Required " + splitdep[1] + ", installing " + db.Packages[splitdep[0]].Package.Version}
					}

					if err := packages.AddEdge(vertex, vertex2); err != nil {
						return err
					}

					continue REQUIRED
				}

				return &ErrorString{S: "Dependency not found: " + dependency}
			}

		OPTIONAL:
			for i := range pkg.Dependencies.Optional {
				dependency := pkg.Dependencies.Optional[i]
				splitdep := strings.Split(dependency, "@")

				if _, ok := db.Packages[splitdep[0]]; ok {
					depVersion, err := semver.NewVersion(db.Packages[splitdep[0]].Package.Version)
					if err != nil {
						return err
					}

					c, err := semver.NewConstraint(splitdep[1])
					if err != nil {
						return err
					}

					if !c.Check(depVersion) {
						return &ErrorString{S: "Version constraint for package " + splitdep[0] + "not met. Required " + splitdep[1] + ", found " + db.Packages[splitdep[0]].Package.Version}
					}

					continue
				}

				for _, file2 := range packageFiles {
					vertex2, err := packages.GetVertex(file2)

					if err != nil {
						return err
					}

					pkg2 := vertex2.Value.(*PackageRoot)

					if pkg2.Package.Name != splitdep[0] {
						continue
					}

					depVersion, err := semver.NewVersion(pkg2.Package.Version)
					if err != nil {
						return err
					}

					c, err := semver.NewConstraint(splitdep[1])
					if err != nil {
						return err
					}

					if !c.Check(depVersion) {
						return &ErrorString{S: "Version constraint for package " + splitdep[0] + "not met. Required " + splitdep[1] + ", installing " + db.Packages[splitdep[0]].Package.Version}
					}

					if err := packages.AddEdge(vertex, vertex2); err != nil {
						return err
					}

					continue OPTIONAL
				}

				return &ErrorString{S: "Dependency not found: " + dependency}
			}
		}

		return nil
	}(); err != nil {
		return nil
	}

	group := new(errgroup.Group)
	state := make(map[string]string)
	var stateLock sync.Mutex
	condLock := sync.Mutex{}
	condLock.Lock()
	cond := sync.NewCond(&condLock)

	entryPoints := packages.SinkVertices()

	for _, point := range entryPoints {
		InstallWorker(root, point, group, state, &stateLock, cond)
	}

	if err := group.Wait(); err != nil {
		return err
	}

	return nil
}

func Remove(root string, packageName string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	installationPath := ""

	if err := func() error {
		dbLock.Lock()
		defer dbLock.Unlock()
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

		installationPath = filepath.Join(root, "packages", db.Packages[packageName].Hash)

		return nil
	}(); err != nil {
		return err
	}

	pkg, err := ParsePackageFile(filepath.Join(installationPath, "package.toml"))

	if err != nil {
		return err
	}

	if pkg.Hooks.Preremove != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Preremove), 0755); err != nil {
			return err
		}
		globalSideEffectLock.Lock()
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Preremove))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			globalSideEffectLock.Unlock()
			return err
		}
		globalSideEffectLock.Unlock()
	}

	if err := RemoveFiles(root, installationPath, pkg); err != nil {
		return err
	}

	if pkg.Hooks.Postremove != "" {
		if err := os.Chmod(filepath.Join(installationPath, pkg.Hooks.Postremove), 0755); err != nil {
			return err
		}
		globalSideEffectLock.Lock()
		cmd := exec.Command(filepath.Join(installationPath, pkg.Hooks.Postremove))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = installationPath

		if err := cmd.Run(); err != nil {
			globalSideEffectLock.Unlock()
			return err
		}
		globalSideEffectLock.Unlock()
	}

	if err := os.RemoveAll(installationPath); err != nil {
		return err
	}

	if err := func() error {
		dbLock.Lock()
		defer dbLock.Unlock()

		db, err := ReadDatabase(root)
		if err != nil {
			return err
		}

		delete(db.Packages, packageName)

		if err := WriteDatabase(root, db); err != nil {
			return err
		}

		return nil
	}(); err != nil {
		return err
	}

	return nil
}

func ListInstalled(root string) (map[string]DBPackage, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}

	dbLock.Lock()
	defer dbLock.Unlock()
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

	dbLock.Lock()
	defer dbLock.Unlock()

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
