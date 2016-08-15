package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
)

func EnsureFile(pathname string) error {
	f, err := os.Open(pathname)
	if err == nil {
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return errors.New("file is a directory")
		}
		return nil
	}

	if !os.IsNotExist(err) {
		// Caught a non-ENOENT error.
		return err
	}

	// File doesn't exist (ENOENT), so create it.
	f, err = os.Create(pathname)
	if err != nil {
		return err
	}

	// Close file descriptor.
	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

func EnsureDir(pathname string) error {
	f, err := os.Open(pathname)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(pathname, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if err := f.Close(); err != nil {
			return err
		}
	}

	fi, err := os.Stat(pathname)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return errors.New("not a directory")
	}

	return nil
}

// The early returns are only for negative or error cases.
func IsEmpty(pathname string) (bool, error) {
	f, err := os.Open(pathname)
	if err != nil {
		return false, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return false, err
	}

	// Test: not a directory, and size greater than zero.
	if !fi.IsDir() && fi.Size() > 0 {
		return false, nil
	}

	// Test: a directory, and ...
	if fi.IsDir() {
		names, err := f.Readdirnames(1)

		// ... no files, unexpected error.
		if names == nil && err != io.EOF {
			return false, err
		}

		// ... files found.
		if len(names) > 0 {
			return false, nil
		}
	}

	return true, nil
}

// Copy file to target dir/file.
func CopyFile(dst, src string) error {
	// Dirty, but works in any Unix.
	return exec.Command("cp", "-p", src, dst).Run()
}

func RealPath(dir string) (string, error) {
	// Save current dir.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(dir); err != nil {
		return "", err
	}

	realDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return realDir, nil
}
