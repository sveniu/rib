package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Filetype enum.
const (
	FILETYPE_FILE = iota
	FILETYPE_DIR
)

// Pathname enum.
const (
	PATHNAME_RIB          = "._RIB_"
	PATHNAME_BUILDD       = "build.d"
	PATHNAME_ROOTFS       = "rootfs"
	PATHNAME_BIN          = "bin"
	PATHNAME_DIST         = "dist"
	PATHNAME_FILES        = "files"
	PATHNAME_TMP          = "tmp"
	PATHNAME_LOG          = "log"
	PATHNAME_FAKEROOTSAVE = "fakeroot.save"
)

// The rib directory skeleton.
var dirSkeleton = []struct {
	pathname string
	filetype int
	envvar   string
	initonly bool
}{
	{PATHNAME_RIB, FILETYPE_FILE, "", true},
	{PATHNAME_BUILDD, FILETYPE_DIR, "RIB_DIR_BUILDD", false},
	{PATHNAME_ROOTFS, FILETYPE_DIR, "RIB_DIR_ROOTFS", false},
	{PATHNAME_BIN, FILETYPE_DIR, "RIB_DIR_BIN", false},
	{PATHNAME_DIST, FILETYPE_DIR, "RIB_DIR_DIST", false},
	{PATHNAME_FILES, FILETYPE_DIR, "RIB_DIR_FILES", false},
	{PATHNAME_TMP, FILETYPE_DIR, "RIB_DIR_TEMP", false},
	{PATHNAME_LOG, FILETYPE_DIR, "RIB_DIR_LOG", false},
	{PATHNAME_FAKEROOTSAVE, FILETYPE_FILE, "", false},
}

// isRibDir checks whether the specified dir is a rib directory by verifying
// the presence of the ._RIB_ file.
func isRibDir(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, PATHNAME_RIB))
	if err != nil {
		return false
	} else {
		if fi.IsDir() {
			return false
		}
	}
	return true
}

// mkDirSkel creates the rib directory skeleton.
func mkDirSkel(root string) error {
	for _, d := range dirSkeleton {
		pathname := filepath.Join(root, d.pathname)
		switch {
		case d.filetype == FILETYPE_FILE:
			if err := EnsureFile(pathname); err != nil {
				return err
			}
		case d.filetype == FILETYPE_DIR:
			if err := EnsureDir(pathname); err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf(
				"unknown file type '%d'", d.filetype))
		}
	}
	return nil
}

// StringInSlice checks whether the given string is present in the list of
// strings.
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// AddSbinEnvPaths appends /sbin and /usr/sbin to the PATH environment variable
// if they are not already present.
func AddSbinEnvPaths() error {
	envPaths := strings.Split(os.Getenv("PATH"), ":")

	if !StringInSlice("/sbin", envPaths) {
		envPaths = append(envPaths, "/sbin")
	}

	if !StringInSlice("/usr/sbin", envPaths) {
		envPaths = append(envPaths, "/usr/sbin")
	}

	return os.Setenv("PATH", strings.Join(envPaths, ":"))
}
