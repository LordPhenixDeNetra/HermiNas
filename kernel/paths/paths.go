// Package paths resolves HermiNas' on-disk layout relative to a single
// root, so the bundle stays relocatable (leçon aNtaerus: no hardcoded
// absolute paths). The layout mirrors the bundle structure documented in
// cahier-des-charges-herminas.md §9.2.
package paths

import (
	"os"
	"path/filepath"
)

type Layout struct {
	Root       string
	ConfigDir  string
	RuntimeDir string
	ModelsDir  string
	DataDir    string
	LogsDir    string
}

func Resolve(root string) Layout {
	return Layout{
		Root:       root,
		ConfigDir:  filepath.Join(root, "config"),
		RuntimeDir: filepath.Join(root, "runtime"),
		ModelsDir:  filepath.Join(root, "models"),
		DataDir:    filepath.Join(root, "data"),
		LogsDir:    filepath.Join(root, "logs"),
	}
}

// Default resolves the layout from HERMINAS_HOME, falling back to the
// current working directory (dev mode; the bundle launcher will always set
// HERMINAS_HOME explicitly, see M8.1).
func Default() Layout {
	root := os.Getenv("HERMINAS_HOME")
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	return Resolve(root)
}
