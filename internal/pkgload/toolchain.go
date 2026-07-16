package pkgload

import (
	"fmt"
	"go/version"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/modfile"
)

// CheckToolchain rejects a target module whose go directive is newer
// than the Go toolchain embedded in the Lagotto binary. Without this
// preflight, go/packages can emit hundreds of misleading syntax and
// type errors when an old release audits newer language features.
func CheckToolchain(root string) error {
	return checkToolchain(root, runtime.Version())
}

func checkToolchain(root, builtWith string) error {
	displayVersion := builtWith
	if fields := strings.Fields(builtWith); len(fields) > 0 {
		builtWith = fields[0]
	}
	goMod, err := nearestGoMod(root)
	if err != nil || goMod == "" {
		return err
	}
	data, err := os.ReadFile(goMod)
	if err != nil {
		return fmt.Errorf("read %s: %w", goMod, err)
	}
	parsed, err := modfile.ParseLax(goMod, data, nil)
	if err != nil {
		return fmt.Errorf("parse %s: %w", goMod, err)
	}
	if parsed.Go == nil || parsed.Go.Version == "" {
		return nil
	}
	required := "go" + parsed.Go.Version
	if !version.IsValid(required) || !version.IsValid(builtWith) {
		return nil
	}
	if version.Compare(builtWith, required) >= 0 {
		return nil
	}
	return fmt.Errorf("target module requires Go %s, but this Lagotto binary was built with %s; install a newer Lagotto release or rebuild it with Go %s or later", parsed.Go.Version, displayVersion, parsed.Go.Version)
}

func nearestGoMod(root string) (string, error) {
	dir, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", root, err)
	}
	for {
		candidate := filepath.Join(dir, "go.mod")
		_, err := os.Stat(candidate)
		switch {
		case err == nil:
			return candidate, nil
		case !os.IsNotExist(err):
			return "", fmt.Errorf("inspect %s: %w", candidate, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
