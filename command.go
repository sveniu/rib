package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Command environment flags.
const (
	Einteractive = 1 << iota
	Efakeroot
	Efakechroot
	Echroot
	Edirectexec
	Eignoreexit
	Eskip
)

// Commands available to child processes.
type ChildData struct {
	category string
	key      string
	value    string
}

// Command execution environment.
type CmdEnv struct {
	exec.Cmd
	flag             int
	workDir          string
	chrootDir        string
	fakerootSaveFile string
	vTmpDir          string
	vExecDir         string
	childDataHandler func(*ChildData)
}

// MakeArgs prepares a command's path and argument vector based on the
// execution environment. It rearranges the arguments to include wrapper
// commands like chroot, fakeroot and fakechroot.
func (ce *CmdEnv) MakeArgs() (err error) {
	if ce.flag&Echroot != 0 {
		if ce.chrootDir == "" {
			return errors.New("chroot dir not defined")
		}
		if ce.Path != "" {
			if ce.Args == nil {
				ce.Args = []string{ce.Path}
			} else {
				ce.Args[0] = ce.Path
			}
		}
		if ce.Path, err = exec.LookPath("chroot"); err != nil {
			return err
		}
		ce.Args = append([]string{ce.Path, ce.chrootDir}, ce.Args...)
	}

	if ce.flag&Efakeroot != 0 {
		if ce.Path, err = exec.LookPath("fakeroot"); err != nil {
			return err
		}
		if ce.fakerootSaveFile != "" {
			ce.Args = append([]string{
				ce.Path,
				"-s", ce.fakerootSaveFile,
				"-i", ce.fakerootSaveFile,
				"--"}, ce.Args...)
		} else {
			ce.Args = append([]string{
				ce.Path,
				"--"}, ce.Args...)
		}
	}

	if ce.flag&Efakechroot != 0 {
		if ce.Path, err = exec.LookPath("fakechroot"); err != nil {
			return err
		}
		ce.Args = append([]string{
			ce.Path,
			"--environment",
			"debootstrap",
			"--",
		}, ce.Args...)
	}

	return nil
}

// MakeVolatileDirs creates volatile directories for a command's execution
// environment.
func (ce *CmdEnv) MakeVolatileDirs() (err error) {
	// Determine target directory for volatile temp dir.
	var vTmpBaseDir string
	if ce.flag&Echroot != 0 {
		vTmpBaseDir = filepath.Join(
			ce.workDir, PATHNAME_ROOTFS)
	} else {
		vTmpBaseDir = filepath.Join(
			ce.workDir, PATHNAME_TMP)
	}

	// Create volatile temp dir.
	ce.vTmpDir, err = ioutil.TempDir(vTmpBaseDir, ".volatile.")
	if err != nil {
		Errorf("ioutil.TempDir: %s", err)
		return err
	}

	if ce.flag&Echroot != 0 {
		// Create volatile execution dir for chroot program.
		ce.vExecDir, err = ioutil.TempDir(vTmpBaseDir, ".exec.")
		if err != nil {
			Errorf("ioutil.TempDir: %s", err)
			return err
		}
	}

	return nil
}

// RemoveVolatileDirs deletes the volatile directories defined by a command's
// execution environment.
func (ce *CmdEnv) RemoveVolatileDirs() {
	for _, dir := range []string{
		ce.vTmpDir,
		ce.vExecDir,
	} {
		if dir != "" {
			os.RemoveAll(dir)
		}
	}
}

// SetEnv configures the command's environment variables based on its execution
// environment.
func (ce *CmdEnv) SetEnv() (err error) {
	// Configure the volatile command environment.
	cmdVolatileEnv := make(map[string]string)
	if ce.flag&Echroot != 0 {
		cmdVolatileEnv["PATH"] = "/usr/sbin:/usr/bin:/sbin:/bin"
		vTmpChrootDir, err := filepath.Rel(
			filepath.Join(ce.workDir, PATHNAME_ROOTFS),
			ce.vTmpDir)
		if err != nil {
			Errorf("filepath.Rel: %s", err)
			return err
		}
		cmdVolatileEnv["VTEMP"] = filepath.Join(
			"/", vTmpChrootDir)
	} else {
		cmdVolatileEnv["PATH"] = fmt.Sprintf("%s:%s",
			filepath.Join(ce.workDir, PATHNAME_BIN),
			"/usr/sbin:/usr/bin:/sbin:/bin")
		cmdVolatileEnv["VTEMP"] = ce.vTmpDir

		// Env vars for the directory skeleton.
		for _, d := range dirSkeleton {
			if d.envvar != "" {
				cmdVolatileEnv[d.envvar] = filepath.Join(
					ce.workDir, d.pathname)
			}
		}
	}

	// Copy volatile environment to ce.Env string slice.
	for name, value := range cmdVolatileEnv {
		ce.Env = append(ce.Env,
			fmt.Sprintf("%s=%s", name, value))
	}

	// Copy persistent environment to ce.Env string slice.
	for name, value := range cmdPersistEnv {
		ce.Env = append(ce.Env,
			fmt.Sprintf("%s=%s", name, value))
	}

	// Always set RIB_EXEC_ENV=1.
	ce.Env = append(ce.Env, "RIB_EXEC_ENV=1")

	return nil
}

// readBuf scans line-based input and sends it to the Debugf logging function.
func readBuf(s *bufio.Scanner, prefix string, stop chan bool) {
	for s.Scan() {
		Debugf("%s %s", prefix, s.Bytes())
	}
	stop <- true
	if err := s.Err(); err != nil {
		Errorf("scan error: %s", err)
	}
}

// scanNull is a split function that returns null-delimited items. It is
// a simple adaption of https://golang.org/pkg/bufio/#ScanLines
func scanNull(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\x00'); i >= 0 {
		// We have a full null-terminated record.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

// readPipe scans input and hands it over to the childDataHandler function
// specified in the command environment.
func readPipe(s *bufio.Scanner, ce *CmdEnv, stop chan bool) {
	for s.Scan() {
		r := bytes.SplitN(s.Bytes(), []byte{'\x1f'}, 3)
		ce.childDataHandler(&ChildData{
			category: string(r[0]),
			key:      string(r[1]),
			value:    string(r[2]),
		})
	}
	stop <- true
	if err := s.Err(); err != nil {
		Errorf("scan error: %s", err)
	}
}

// RunCmd executes the command according to its environment. An interactive
// command will run with stdin/out/err connected to the current terminal;
// a non-interactive command will have its stdout/err captured and logged.
func (ce *CmdEnv) RunCmd() error {
	var err error

	// Set chroot directory.
	ce.chrootDir = filepath.Join(ce.workDir, PATHNAME_ROOTFS)

	// Set fakeroot save file path.
	ce.fakerootSaveFile = filepath.Join(ce.workDir, PATHNAME_FAKEROOTSAVE)

	// Set up volatile directories.
	if err := ce.MakeVolatileDirs(); err != nil {
		return err
	}
	defer ce.RemoveVolatileDirs()

	if ce.flag&Echroot != 0 && ce.flag&Edirectexec == 0 {
		// Copy program to in-chroot, temporary execution dir.
		Debugf("Copying '%s' to '%s'.", ce.Path, ce.vExecDir)
		if err := CopyFile(ce.vExecDir, ce.Path); err != nil {
			Errorf("CopyFile: %s", err)
			return err
		}

		// Modify Path to be relative to the chroot dir.
		ce.Path = filepath.Join("/",
			filepath.Base(ce.vExecDir),
			filepath.Base(ce.Path))
	}

	// Set up command environment.
	if err := ce.SetEnv(); err != nil {
		return err
	}

	// Set up command arguments.
	if err := ce.MakeArgs(); err != nil {
		return err
	}

	// Configure IO pipe, which the child can use to write data
	// back to the main process.
	pipeReadFile, pipeWriteFile, err := os.Pipe()
	if err != nil {
		Errorf("os.Pipe: %s", err)
		return err
	}
	defer pipeReadFile.Close()
	pipeScanner := bufio.NewScanner(pipeReadFile)
	pipeScanner.Split(scanNull)
	stopPipe := make(chan bool)
	go readPipe(pipeScanner, ce, stopPipe)
	ce.ExtraFiles = []*os.File{pipeWriteFile}

	Infof("Executing command: %s %s",
		ce.Path, strings.Join(ce.Args[1:], " "))

	if ce.flag&Einteractive != 0 {
		ce.Stdin = os.Stdin
		ce.Stdout = os.Stdout
		ce.Stderr = os.Stderr

		err = ce.Start()
		if err != nil {
			Errorf("ce.Start: %s", err)
			return err
		}

		// Close our copy of the pipe's write end, to make our
		// scanner's read call return EOF. Ref pipe(7).
		if err := pipeWriteFile.Close(); err != nil {
			Errorf("Close: %s", err)
			return err
		}

		<-stopPipe
		err = ce.Wait()
	} else {
		// Capture stdout and stderr.
		var cmdStdoutReader, cmdStderrReader io.ReadCloser
		cmdStdoutReader, err = ce.StdoutPipe()
		if err != nil {
			Errorf("Error creating StdoutPipe for Cmd: %s", err)
			return err
		}

		cmdStderrReader, err = ce.StderrPipe()
		if err != nil {
			Errorf("Error creating StderrPipe for Cmd: %s", err)
			return err
		}

		err = ce.Start()
		if err != nil {
			Errorf("ce.Start: %s", err)
			return err
		}

		stdoutScanner := bufio.NewScanner(cmdStdoutReader)
		stopStdout := make(chan bool)
		go readBuf(stdoutScanner, "[stdout]", stopStdout)

		stderrScanner := bufio.NewScanner(cmdStderrReader)
		stopStderr := make(chan bool)
		go readBuf(stderrScanner, "[stderr]", stopStderr)

		// Close our copy of the pipe's write end to make our
		// scanner's read call return EOF, ref pipe(7).
		if err := pipeWriteFile.Close(); err != nil {
			Errorf("Close: %s", err)
		}

		<-stopStdout
		<-stopStderr
		<-stopPipe
		err = ce.Wait()
	}

	if err != nil && ce.flag&Eignoreexit != 0 {
		Warningf("Ignoring '%s' error: %s", ce.Path, err)
		err = nil
	}
	return err
}

// PrepareParts returns a list of command environments based on build scripts
// found in the given directory. Each script's sequence number must be equal to
// or greater than the given seqmin value. Flags are parsed from the script
// filename, and decide how the command environment struct is configured.
func PrepareParts(dir string, seqmin int) (celist []*CmdEnv, err error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Match filenames containing a sequence number and a list of
	// execution flags, followed by an arbitrary name.
	re := regexp.MustCompile(`^(\d+)-([A-Z]*)-`)
	for _, file := range files {
		ce := &CmdEnv{}
		ce.Path = filepath.Join(dir, file.Name())
		ce.Args = []string{ce.Path}

		groups := re.FindStringSubmatch(file.Name())
		if len(groups) != 3 {
			Warningf("Skipping file '%s': regex mismatch",
				file.Name())
			continue
		}

		// Parse sequence number and compare to seqmin.
		seq, err := strconv.Atoi(groups[1])
		if err != nil {
			Errorf("strconv.Atoi: %s", err)
			continue
		}
		if seq < seqmin {
			Warningf("Skipping file '%s': seqno=%d < seqmin=%d",
				file.Name(), seq, seqmin)
			continue
		}

		// Parse execution flags.
		for _, flag := range groups[2] {
			switch {
			case flag == 'I':
				ce.flag |= Einteractive
			case flag == 'R':
				ce.flag |= Efakeroot
			case flag == 'F':
				ce.flag |= Efakechroot
			case flag == 'C':
				ce.flag |= Echroot
				ce.flag |= Efakeroot
				ce.flag |= Efakechroot
			case flag == 'E':
				ce.flag |= Eignoreexit
			case flag == 'S':
				ce.flag |= Eskip
			default:
				Warningf("Ignoring unknown flag %q.", flag)
			}
		}

		if ce.flag&Eskip != 0 {
			continue
		}

		Debugf("Registering build command: %s", ce.Path)
		celist = append(celist, ce)
	}

	return celist, nil
}
