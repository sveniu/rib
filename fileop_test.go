package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "test.fileop.")
	if err != nil {
		t.Fatalf("Failed to make temp dir: %s", err)
	}
	defer os.RemoveAll(dir)

	// Test on a non-existing file.
	tmpfn := filepath.Join(dir, "new")
	err = EnsureFile(tmpfn)
	if err != nil {
		t.Fatalf("EnsureFile() failed: %s", err)
	}

	fi, err := os.Stat(tmpfn)
	if os.IsNotExist(err) {
		t.Fatalf("Ensured file '%s' missing: %s", tmpfn, err)
	}

	if !fi.Mode().IsRegular() {
		t.Fatalf("Ensured file '%s' is not a regular file.")
	}

	// Test on a pre-existing file.
	tmpfn = filepath.Join(dir, "prev")
	f, err := os.Create(tmpfn)
	if err != nil {
		t.Fatalf("Could not create test file '%s': %s", tmpfn, err)
	}
	f.Close()

	err = EnsureFile(tmpfn)
	if err != nil {
		t.Fatalf("EnsureFile() failed: %s", err)
	}

	fi, err = os.Stat(tmpfn)
	if os.IsNotExist(err) {
		t.Fatalf("Ensured file '%s' missing: %s", tmpfn, err)
	}

	if !fi.Mode().IsRegular() {
		t.Fatalf("Ensured file '%s' is not a regular file.")
	}

	// Test ENAMETOOLONG. A megabyte-size file name should exceed
	// the limit set by any system.
	tmpfn = filepath.Join(dir, strings.Repeat("a", 1024*1024))
	err = EnsureFile(tmpfn)
	if err == nil {
		t.Fatalf("EnsureFile(<1MB-long filename>) did not fail.")
	}

	// Put a directory in place of the ensured file.
	tmpfn = filepath.Join(dir, "dir")
	if err = os.Mkdir(tmpfn, 0755); err != nil {
		t.Fatalf("Could not mkdir(%s): %s", dir, err)
	}
	err = EnsureFile(tmpfn)
	if err == nil {
		t.Fatalf("EnsureFile(directory) did not fail.")
	}

	// Ensure a deep path. This should usually work, since
	// os.Create() calls openat() with O_CREAT, which will happily
	// create a file in a deep directory structure.
	tmpfn = filepath.Join(dir, "dir", "file")
	err = EnsureFile(tmpfn)
	if err != nil {
		t.Fatalf("EnsureFile(dir/file) failed: %s", err)
	}

	// Ensure an invalid path.
	tmpfn = filepath.Join("/", "proc", "file")
	err = EnsureFile(tmpfn)
	if err == nil {
		t.Fatalf("EnsureFile(/proc/file) did not fail.")
	}
}

func TestEnsureDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "test.fileop.")
	if err != nil {
		t.Fatalf("Failed to make temp dir: %s", err)
	}
	defer os.RemoveAll(dir)

	// Test on a non-existing dir.
	tmpfn := filepath.Join(dir, "new")
	err = EnsureDir(tmpfn)
	if err != nil {
		t.Fatalf("EnsureDir() failed: %s", err)
	}

	fi, err := os.Stat(tmpfn)
	if os.IsNotExist(err) {
		t.Fatalf("Ensured dir '%s' missing: %s", tmpfn, err)
	}

	if !fi.IsDir() {
		t.Fatalf("Ensured dir '%s' is not a directory.")
	}

	// Test on a pre-existing file.
	tmpfn = filepath.Join(dir, "prev")
	err = os.Mkdir(tmpfn, 0755)
	if err != nil {
		t.Fatalf("Could not create test dir '%s': %s", tmpfn, err)
	}

	err = EnsureDir(tmpfn)
	if err != nil {
		t.Fatalf("EnsureDir() failed: %s", err)
	}

	fi, err = os.Stat(tmpfn)
	if os.IsNotExist(err) {
		t.Fatalf("Ensured dir '%s' missing: %s", tmpfn, err)
	}

	if !fi.IsDir() {
		t.Fatalf("Ensured dir '%s' is not a directory.")
	}

	// Test ENAMETOOLONG. A megabyte-size file name should exceed
	// the limit set by any system.
	tmpfn = filepath.Join(dir, strings.Repeat("a", 1024*1024))
	err = EnsureDir(tmpfn)
	if err == nil {
		t.Fatalf("EnsureDir(<1MB-long filename>) did not fail.")
	}

	// Put a file in place of the ensured dir.
	tmpfn = filepath.Join(dir, "file")
	f, err := os.Create(tmpfn)
	if err != nil {
		t.Fatalf("Could not create file: %s", err)
	}
	f.Close()
	err = EnsureDir(tmpfn)
	if err == nil {
		t.Fatalf("EnsureDir(file) did not fail.")
	}

	// Ensure a deep path.
	tmpfn = filepath.Join(dir, "dir", "dir")
	err = EnsureDir(tmpfn)
	if err != nil {
		t.Fatalf("EnsureDir(dir/dir) failed: %s", err)
	}

	fi, err = os.Stat(tmpfn)
	if os.IsNotExist(err) {
		t.Fatalf("Ensured dir '%s' missing: %s", tmpfn, err)
	}

	if !fi.IsDir() {
		t.Fatalf("Ensured dir '%s' is not a directory.")
	}

	// Ensure an invalid path.
	tmpfn = filepath.Join("/", "proc", "dir")
	err = EnsureDir(tmpfn)
	if err == nil {
		t.Fatalf("EnsureDir(/proc/dir) did not fail.")
	}
}

func TestIsEmpty(t *testing.T) {
	var (
		tmpfn string
		empty bool
		err   error
	)

	dir, err := ioutil.TempDir("", "test.fileop.")
	if err != nil {
		t.Fatalf("Failed to make temp dir: %s", err)
	}
	defer os.RemoveAll(dir)

	// Test on invalid file.
	tmpfn = "/proc/abcdefghijklmnopqrstuvwxyz"
	empty, err = IsEmpty(tmpfn)
	if err == nil {
		t.Fatalf("IsEmpty(%s) succeeded.", tmpfn)
	}

	// Test on an empty file.
	tmpfn = filepath.Join(dir, "file.empty")
	err = EnsureFile(tmpfn)
	if err != nil {
		t.Fatalf("EnsureFile(%s) failed: %s", tmpfn, err)
	}

	empty, err = IsEmpty(tmpfn)
	if err != nil {
		t.Fatalf("IsEmpty(%s) failed: %s", tmpfn, err)
	}

	if !empty {
		t.Fatalf("IsEmpty(empty file) returned false.", tmpfn)
	}

	// Test on a non-empty file.
	tmpfn = filepath.Join(dir, "file.non-empty")
	srcdata := []byte("test")
	if err = ioutil.WriteFile(tmpfn, srcdata, 0644); err != nil {
		t.Fatalf("WriteFile to '%s' failed: %s", tmpfn, err)
	}

	empty, err = IsEmpty(tmpfn)
	if err != nil {
		t.Fatalf("IsEmpty(%s) failed: %s", tmpfn, err)
	}

	if empty {
		t.Fatalf("IsEmpty(non-empty file) returned true.", tmpfn)
	}

	// Test on an empty directory.
	tmpfn = filepath.Join(dir, "file")
	err = EnsureFile(tmpfn)
	if err != nil {
		t.Fatalf("Ensurefile(%s) failed: %s", tmpfn, err)
	}

	empty, err = IsEmpty(tmpfn)
	if err != nil {
		t.Fatalf("IsEmpty(%s) failed: %s", tmpfn, err)
	}

	if !empty {
		t.Fatalf("IsEmpty(empty dir) returned false.", tmpfn)
	}

	// Test on a non-empty directory.
	tmpfn = filepath.Join(dir, "dir")
	err = EnsureDir(tmpfn)
	if err != nil {
		t.Fatalf("EnsureDir(%s) failed: %s", tmpfn, err)
	}

	err = EnsureFile(filepath.Join(tmpfn, "file"))
	if err != nil {
		t.Fatalf("EnsureFile(%s) failed: %s", filepath.Join(tmpfn, "file"), err)
	}

	empty, err = IsEmpty(tmpfn)
	if err != nil {
		t.Fatalf("IsEmpty(%s) failed: %s", tmpfn, err)
	}

	if empty {
		t.Fatalf("IsEmpty(non-empty dir) returned false.", tmpfn)
	}
}

func TestCopyFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "test.fileop.")
	if err != nil {
		t.Fatalf("Failed to make temp dir: %s", err)
	}
	defer os.RemoveAll(dir)

	// Create a source file.
	srcdata := []byte("test")
	srcfn := filepath.Join(dir, "src")
	if err := ioutil.WriteFile(srcfn, srcdata, 0644); err != nil {
		t.Fatalf("WriteFile to '%s' failed: %s", srcfn, err)
	}

	dstfn := filepath.Join(dir, "dst")
	if err = CopyFile(dstfn, srcfn); err != nil {
		t.Fatalf("CopyFile from '%s' to '%s' failed: %s",
			srcfn, dstfn, err)
	}

	dstdata, err := ioutil.ReadFile(dstfn)
	if err != nil {
		t.Fatalf("ReadFile of '%s' failed: %s", dstfn, err)
	}

	if !bytes.Equal(srcdata, dstdata) {
		t.Fatalf("Content mismatch: %s != %s", dstdata, srcdata)
	}
}
