package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

// Map of persistent environment variables exported to all commands.
var cmdPersistEnv map[string]string

func cmdInit(workDir string) error {
	// Create target dir if missing.
	if err := EnsureDir(workDir); err != nil {
		Errorf("EnsureDir failed: %s", err)
		return err
	}

	if isRibDir(workDir) {
		Errorf("Directory '%s' already initialized.", workDir)
		return errors.New("already initialized")
	}

	// Verify that target dir is empty.
	empty, err := IsEmpty(workDir)
	if err != nil {
		return err
	}
	if !empty {
		Errorf("Directory '%s' is not empty.", workDir)
		return errors.New("not empty")
	}

	if err := mkDirSkel(workDir); err != nil {
		Errorf("mkDirSkel(%s) failed: %s", workDir, err)
		return err
	}

	return nil
}

func handleChildData(cd *ChildData) {
	switch {
	case cd.category == "setenv":
		// Add to the persistent command environment.
		cmdPersistEnv[cd.key] = cd.value
	case cd.category == "unsetenv":
		// Remove from the persistent command environment.
		delete(cmdPersistEnv, cd.key)
	}
}

func cmdBuild(workDir string, seqmin int) error {
	workDir, err := RealPath(workDir)
	if err != nil {
		Errorf("RealPath: %s")
		return err
	}

	if !isRibDir(workDir) {
		Errorf("Directory '%s' not initialized.", workDir)
		return errors.New("directory not initialized")
	}

	if err := mkDirSkel(workDir); err != nil {
		Errorf("mkDirSkel(%s) failed: %s", workDir, err)
		return err
	}

	// Open log file.
	f, err := os.OpenFile(
		filepath.Join(workDir, PATHNAME_LOG, "build.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0600)
	if err != nil {
		return err
	}
	AddLoggerOutput(f)

	// Initialize the persistent command environment.
	cmdPersistEnv = make(map[string]string)

	// Start timer.
	t0 := time.Now()

	// Prepare execution parts.
	celist, err := PrepareParts(filepath.Join(workDir, PATHNAME_BUILDD),
		seqmin)
	if err != nil {
		Infof("PrepareParts: %s", err)
		return err
	}

	if len(celist) == 0 {
		Warningf("No build scripts found in '%s'.", filepath.Join(
			workDir, PATHNAME_BUILDD))
		return nil
	}

	// Iterate over each command execution environment.
	for _, ce := range celist {
		ce.workDir = workDir
		ce.childDataHandler = handleChildData

		if err := ce.RunCmd(); err != nil {
			Errorf("Command failed: %s", err)
			return err
		}
	}

	t1 := time.Now()
	Infof("Build duration: %s", t1.Sub(t0).String())

	return nil
}

func cmdShell(workDir string, args []string) error {
	workDir, err := RealPath(workDir)
	if err != nil {
		Errorf("RealPath: %s")
		return err
	}

	if !isRibDir(workDir) {
		Errorf("No rib structure found in '%s'.", workDir)
		return errors.New("invalid directory")
	}

	ce := &CmdEnv{
		workDir: workDir,
		chrootDir: filepath.Join(
			workDir, PATHNAME_ROOTFS),
		fakerootSaveFile: filepath.Join(
			workDir, PATHNAME_FAKEROOTSAVE),
	}
	ce.flag |= Einteractive |
		Echroot |
		Edirectexec |
		Efakeroot |
		Efakechroot

	if len(args) > 0 {
		ce.Path = args[0]
		ce.Args = args
	} else {
		// Prepare a simple bash rc file.
		f, err := ioutil.TempFile(filepath.Join(
			workDir, PATHNAME_ROOTFS), ".volatile.bashrc.")
		if err != nil {
			return err
		}
		if _, err := f.Write([]byte(
			`alias ls='ls --color=auto'` + "\n" +
				`HISTFILE=""` + "\n" +
				`PS1='\u@[rib]:\w\$ '` + "\n")); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		defer os.Remove(f.Name())
		bashrcRelPath, err := filepath.Rel(filepath.Join(
			workDir, PATHNAME_ROOTFS), f.Name())
		if err != nil {
			return err
		}
		bashrcRelPath = filepath.Join("/", bashrcRelPath)
		Infof("bashrcRelPath: %s\n", bashrcRelPath)

		ce.Path = "/bin/bash"
		ce.Args = []string{
			"bash",
			"--rcfile",
			bashrcRelPath,
			"-i",
		}
	}
	if err := ce.RunCmd(); err != nil {
		Infof("ce.RunCmd: %s", err)
	}

	return nil
}

func cmdClean(workDir string, all bool) error {
	workDir, err := RealPath(workDir)
	if err != nil {
		Errorf("RealPath: %s")
		return err
	}

	if !isRibDir(workDir) {
		Errorf("No rib structure found in '%s'.", workDir)
		return errors.New("invalid directory")
	}

	// Default cleanup targets.
	targets := []string{
		PATHNAME_ROOTFS,
		PATHNAME_TMP,
		PATHNAME_FAKEROOTSAVE,
	}

	if all {
		targets = append(targets,
			PATHNAME_DIST,
			PATHNAME_LOG,
		)
	}

	// Delete targets.
	for _, target := range targets {
		pathname := filepath.Join(workDir, target)
		Debugf("Removing '%s'.", pathname)
		if err := os.RemoveAll(pathname); err != nil {
			Errorf("os.RemoveAll(%s): %s", pathname, err)
			return err
		}
	}

	// Ensure a consistent directory skeleton.
	if err := mkDirSkel(workDir); err != nil {
		Errorf("mkDirSkel(%s) failed: %s", workDir, err)
		return err
	}

	return nil
}

func main() {
	// Kingpin configuration.
	var (
		app     = kingpin.New("rib", "Root Image Build tool.")
		verbose = app.Flag("verbose", "Enable verbose output.").Short('v').Counter()
		quiet   = app.Flag("quiet", "Enable quiet output.").Short('q').Bool()
		dir     = app.Flag("dir", "Work directory.").Default(".").Short('d').String()

		init    = app.Command("init", "Create empty rib directory.")
		initdir = init.Arg("workdir", "Work directory.").String()

		build    = app.Command("build", "Run build scripts.")
		buildseq = build.Flag("buildseq", "Minimum sequence number.").Short('s').Default("0").Int()

		shell     = app.Command("shell", "Run build scripts.")
		shellargs = shell.Arg("shellargs", "Command args.").Strings()

		clean    = app.Command("clean", "Clean rootfs, tmp and fakeroot.save.")
		cleanall = clean.Flag("all", "Also clean dist and log directories.").Short('a').Bool()
	)

	// Don't run as root.
	user, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "User lookup failed: %s", err)
		os.Exit(1)
	}
	if user.Uid == "0" || user.Name == "root" {
		fmt.Fprintf(os.Stderr, "Cannot run as root.\n")
		os.Exit(1)
	}

	// Configure PATH.
	AddSbinEnvPaths()

	// Parse command line.
	app.HelpFlag.Short('h')
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	// Configure logging.
	slog := NewLogger(ioutil.Discard, "", 0)
	slog.SetStandard()
	if *quiet {
		os.Stdout = nil
		os.Stderr = nil
	} else {
		if *verbose >= 1 {
			slog.AddOutput(os.Stderr)
		}
		if *verbose >= 2 {
			slog.EnableDebug()
		}
	}

	// Set workDir from the global --dir option, optionally
	// overridden by the first argument to the 'init' subcommand.
	workDir := *dir
	if *initdir != "" {
		workDir = *initdir
	}

	switch cmd {
	case init.FullCommand():
		if err := cmdInit(workDir); err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to initialize '%s': %s\n",
				workDir, err)
			os.Exit(1)
		} else {
			fmt.Printf("Initialized directory '%s'.\n", workDir)
		}
	case build.FullCommand():
		if err := cmdBuild(workDir, *buildseq); err != nil {
			fmt.Fprintf(os.Stderr,
				"Build failed: %s\n", err)
			os.Exit(1)
		}
	case shell.FullCommand():
		if err := cmdShell(workDir, *shellargs); err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to execute shell: %s\n", err)
			os.Exit(1)
		}
	case clean.FullCommand():
		if err := cmdClean(workDir, *cleanall); err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to clean: %s\n", err)
			os.Exit(1)
		}
	}
}
