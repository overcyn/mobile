package matcha

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func Init(flags *Flags) error {
	start := time.Now()

	// Get $GOPATH/pkg/gomobile
	gomobilepath, err := GoMobilePath()
	if err != nil {
		return err
	}
	if flags.ShouldPrint() {
		fmt.Fprintln(os.Stderr, "GOMOBILE="+gomobilepath)
	}

	// Delete $GOPATH/pkg/gomobile
	if err := RemoveAll(flags, gomobilepath); err != nil {
		return err
	}

	// Make $GOPATH/pkg/gomobile
	if err := Mkdir(flags, gomobilepath); err != nil {
		return err
	}

	// Make $GOPATH/pkg/gomobile/work...
	tmpdir, err := NewTmpDir(flags, gomobilepath)
	if err != nil {
		return err
	}
	defer RemoveAll(flags, tmpdir)

	// Install standard libraries for cross compilers.
	var env []string
	if env, err = DarwinArmEnv(flags); err != nil {
		return err
	}
	if err := InstallPkg(flags, tmpdir, "std", env); err != nil {
		return err
	}

	if env, err = DarwinArm64Env(flags); err != nil {
		return err
	}
	if err := InstallPkg(flags, tmpdir, "std", env); err != nil {
		return err
	}

	if env, err = DarwinAmd64Env(flags); err != nil {
		return err
	}
	if err := InstallPkg(flags, tmpdir, "std", env, "-tags=ios"); err != nil {
		return err
	}

	// Write Go Version to $GOPATH/pkg/gomobile/version
	verpath := filepath.Join(gomobilepath, "version")
	if flags.ShouldPrint() {
		Printcmd("go version > %s", verpath)
	}
	if flags.ShouldRun() {
		goversion, err := GoVersion(flags)
		if err != nil {
			return nil
		}
		if err := ioutil.WriteFile(verpath, goversion, 0644); err != nil {
			return err
		}
	}

	// Timing
	if flags.BuildV {
		took := time.Since(start) / time.Second * time.Second
		fmt.Fprintf(os.Stderr, "\nDone, build took %s.\n", took)
	}
	return nil
}
