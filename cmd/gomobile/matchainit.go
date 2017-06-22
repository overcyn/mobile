package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func matchaGoVersion() ([]byte, error) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		return nil, errors.New("go not found")
	}
	goVer, err := exec.Command(gobin, "version").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("'go version' failed: %v, %s", err, goVer)
	}
	switch {
	case bytes.HasPrefix(goVer, []byte("go version go1.4")),
		bytes.HasPrefix(goVer, []byte("go version go1.5")),
		bytes.HasPrefix(goVer, []byte("go version go1.6")):
		return nil, errors.New("Go 1.7 or newer is required")
	}
	return goVer, nil
}

// var (
//     goos    = runtime.GOOS
//     goarch  = runtime.GOARCH
//     ndkarch string
// )

// func init() {
// 	switch runtime.GOARCH {
// 	case "amd64":
// 		ndkarch = "x86_64"
// 	case "386":
// 		ndkarch = "x86"
// 	default:
// 		ndkarch = runtime.GOARCH
// 	}
// }

var cmdMatchaInit = &command{
	run:   runInitMatcha,
	Name:  "matchainit",
	Usage: "[-u]",
	Short: "install mobile compiler toolchain",
	Long: `
Init builds copies of the Go standard library for mobile devices.
It uses Xcode, if available, to build for iOS and uses the Android
NDK from the ndk-bundle SDK package or from the -ndk flag, to build
for Android.
If a OpenAL source directory is specified with -openal, init will
also build an Android version of OpenAL for use with gomobile build
and gomobile install.
`,
}

// var (
// 	initNDK    string // -ndk
// 	initOpenAL string // -openal
// )

// func init() {
// 	cmdInit.flag.StringVar(&initNDK, "ndk", "", "Android NDK path")
// 	cmdInit.flag.StringVar(&initOpenAL, "openal", "", "OpenAL source path")
// }

func runInitMatcha(cmd *command) error {
	// Get GOPATH
	gopaths := filepath.SplitList(matchaGoEnv("GOPATH"))
	if len(gopaths) == 0 {
		return fmt.Errorf("GOPATH is not set")
	}
	gomobilepath = filepath.Join(gopaths[0], "pkg/gomobile")

	// Delete $GOPATH/pkg/gomobile
	verpath := filepath.Join(gomobilepath, "version")
	if buildX || buildN {
		fmt.Fprintln(xout, "GOMOBILE="+gomobilepath)
	}
	matchaRemoveAll(gomobilepath)

	// Make $GOPATH/pkg/gomobile
	if err := mkdir(gomobilepath); err != nil {
		return err
	}

	// Make $GOPATH/pkg/work
	if buildN {
		tmpdir = filepath.Join(gomobilepath, "work")
	} else {
		var err error
		tmpdir, err = ioutil.TempDir(gomobilepath, "work-")
		if err != nil {
			return err
		}
	}
	// if buildX || buildN {
	// 	fmt.Fprintln(xout, "WORK="+tmpdir)
	// }
	defer func() {
		// if buildWork {
		// 	fmt.Printf("WORK=%s\n", tmpdir)
		// 	return
		// }
		matchaRemoveAll(tmpdir)
	}()

	// // Build NDK stuff?
	// if buildN {
	// 	initNDK = "$NDK_PATH"
	// 	initOpenAL = "$OPENAL_PATH"
	// } else {
	// 	toolsDir := filepath.Join("prebuilt", archNDK(), "bin")
	// 	// Try the ndk-bundle SDK package package, if installed.
	// 	if initNDK == "" {
	// 		if sdkHome := os.Getenv("ANDROID_HOME"); sdkHome != "" {
	// 			path := filepath.Join(sdkHome, "ndk-bundle")
	// 			if st, err := os.Stat(filepath.Join(path, toolsDir)); err == nil && st.IsDir() {
	// 				initNDK = path
	// 			}
	// 		}
	// 	}
	// 	if initNDK != "" {
	// 		var err error
	// 		if initNDK, err = filepath.Abs(initNDK); err != nil {
	// 			return err
	// 		}
	// 		// Check if the platform directory contains a known subdirectory.
	// 		if _, err := os.Stat(filepath.Join(initNDK, toolsDir)); err != nil {
	// 			if os.IsNotExist(err) {
	// 				return fmt.Errorf("%q does not point to an Android NDK.", initNDK)
	// 			}
	// 			return err
	// 		}
	// 		ndkFile := filepath.Join(gomobilepath, "android_ndk_root")
	// 		if err := ioutil.WriteFile(ndkFile, []byte(initNDK), 0644); err != nil {
	// 			return err
	// 		}
	// 	}
	// 	if initOpenAL != "" {
	// 		var err error
	// 		if initOpenAL, err = filepath.Abs(initOpenAL); err != nil {
	// 			return err
	// 		}
	// 	}
	// }
	// ndkRoot = initNDK
	if err := envInit(); err != nil {
		return err
	}

	// // Install "golang.org/x/mobile/gl", "golang.org/x/mobile/app", "golang.org/x/mobile/exp/app/debug",
	// if runtime.GOOS == "darwin" {
	// 	// Install common x/mobile packages for local development.
	// 	// These are often slow to compile (due to cgo) and easy to forget.
	// 	//
	// 	// Limited to darwin for now as it is common for linux to
	// 	// not have GLES installed.
	// 	//
	// 	// TODO: consider testing GLES installation and suggesting it here
	// 	for _, pkg := range commonPkgs {
	// 		if err := installPkg(pkg, nil); err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	// Install standard libraries for cross compilers.
	start := time.Now()
	// Ideally this would be -buildmode=c-shared.
	// https://golang.org/issue/13234.
	androidArgs := []string{"-gcflags=-shared", "-ldflags=-shared"}
	for _, arch := range archs {
		env := androidEnv[arch]
		if err := matchaInstallPkg("std", env, gomobilepath, androidArgs...); err != nil {
			return err
		}
	}

	// Install iOS libraries
	if err := matchaInstallDarwin(); err != nil {
		return err
	}

	// Write Go Version to $GOPATH/pkg/gomobile/version
	if buildX || buildN {
		matchaprintcmd("go version > %s", verpath)
	}
	if !buildN {
		goversion, err := matchaGoVersion()
		if err != nil {
			return nil
		}
		if err := ioutil.WriteFile(verpath, goversion, 0644); err != nil {
			return err
		}
	}
	if buildV {
		took := time.Since(start) / time.Second * time.Second
		fmt.Fprintf(os.Stderr, "\nDone, build took %s.\n", took)
	}
	return nil
}

func matchaInstallDarwin() error {
	if !matchaXcodeAvailable() {
		return nil
	}
	if err := matchaInstallPkg("std", darwinArmEnv, gomobilepath); err != nil {
		return err
	}
	if err := matchaInstallPkg("std", darwinArm64Env, gomobilepath); err != nil {
		return err
	}
	// TODO(crawshaw): darwin/386 for the iOS simulator?
	if err := matchaInstallPkg("std", darwinAmd64Env, gomobilepath, "-tags=ios"); err != nil {
		return err
	}
	return nil
}

func matchaInstallPkg(pkg string, env []string, matchaGoMobilePath string, args ...string) error {
	tOS, tArch, pd := matchaGetenv(env, "GOOS"), matchaGetenv(env, "GOARCH"), matchapkgdir(matchaGoMobilePath, env)
	if tOS != "" && tArch != "" {
		if buildV {
			fmt.Fprintf(os.Stderr, "\n# Installing %s for %s/%s.\n", pkg, tOS, tArch)
		}
		args = append(args, "-pkgdir="+pd)
	} else {
		if buildV {
			fmt.Fprintf(os.Stderr, "\n# Installing %s.\n", pkg)
		}
	}

	cmd := exec.Command("go", "install")
	cmd.Args = append(cmd.Args, args...)
	if buildV {
		cmd.Args = append(cmd.Args, "-v")
	}
	if buildX {
		cmd.Args = append(cmd.Args, "-x")
	}
	if buildWork {
		cmd.Args = append(cmd.Args, "-work")
	}
	cmd.Args = append(cmd.Args, pkg)
	cmd.Env = append([]string{}, env...)
	return matchaRunCmd(cmd)
}

// func mkdir(dir string) error {
// 	if buildX || buildN {
// 		matchaprintcmd("mkdir -p %s", dir)
// 	}
// 	if buildN {
// 		return nil
// 	}
// 	return os.MkdirAll(dir, 0755)
// }

// func symlink(src, dst string) error {
// 	if buildX || buildN {
// 		matchaprintcmd("ln -s %s %s", src, dst)
// 	}
// 	if buildN {
// 		return nil
// 	}
// 	if goos == "windows" {
// 		return doCopyAll(dst, src)
// 	}
// 	return os.Symlink(src, dst)
// }

// func rm(name string) error {
// 	if buildX || buildN {
// 		matchaprintcmd("rm %s", name)
// 	}
// 	if buildN {
// 		return nil
// 	}
// 	return os.Remove(name)
// }

// func doCopyAll(dst, src string) error {
// 	return filepath.Walk(src, func(path string, info os.FileInfo, errin error) (err error) {
// 		if errin != nil {
// 			return errin
// 		}
// 		prefixLen := len(src)
// 		if len(path) > prefixLen {
// 			prefixLen++ // file separator
// 		}
// 		outpath := filepath.Join(dst, path[prefixLen:])
// 		if info.IsDir() {
// 			return os.Mkdir(outpath, 0755)
// 		}
// 		in, err := os.Open(path)
// 		if err != nil {
// 			return err
// 		}
// 		defer in.Close()
// 		out, err := os.OpenFile(outpath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode())
// 		if err != nil {
// 			return err
// 		}
// 		defer func() {
// 			if errc := out.Close(); err == nil {
// 				err = errc
// 			}
// 		}()
// 		_, err = io.Copy(out, in)
// 		return err
// 	})
// }

func matchaRemoveAll(path string) error {
	if buildX || buildN {
		matchaprintcmd(`rm -r -f "%s"`, path)
	}
	if buildN {
		return nil
	}

	return os.RemoveAll(path)
}

func matchaGoEnv(name string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}
	val, err := exec.Command("go", "env", name).Output()
	if err != nil {
		panic(err) // the Go tool was tested to work earlier
	}
	return strings.TrimSpace(string(val))
}

func matchaRunCmd(cmd *exec.Cmd) error {
	if buildX || buildN {
		dir := ""
		if cmd.Dir != "" {
			dir = "PWD=" + cmd.Dir + " "
		}
		env := strings.Join(cmd.Env, " ")
		if env != "" {
			env += " "
		}
		matchaprintcmd("%s%s%s", dir, env, strings.Join(cmd.Args, " "))
	}

	buf := new(bytes.Buffer)
	buf.WriteByte('\n')
	if buildV {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = buf
		cmd.Stderr = buf
	}

	if buildWork {
		if goos == "windows" {
			cmd.Env = append(cmd.Env, `TEMP=`+tmpdir)
			cmd.Env = append(cmd.Env, `TMP=`+tmpdir)
		} else {
			cmd.Env = append(cmd.Env, `TMPDIR=`+tmpdir)
		}
	}

	if !buildN {
		cmd.Env = matchaEnviron(cmd.Env)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %v%s", strings.Join(cmd.Args, " "), err, buf)
		}
	}
	return nil
}

func matchaXcodeAvailable() bool {
	_, err := exec.LookPath("xcrun")
	return err == nil
}

func matchaGetenv(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return kv[len(prefix):]
		}
	}
	return ""
}

func matchaprintcmd(format string, args ...interface{}) {
	cmd := fmt.Sprintf(format+"\n", args...)
	// if tmpdir != "" {
	// 	cmd = strings.Replace(cmd, tmpdir, "$WORK", -1)
	// }
	// if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
	// 	cmd = strings.Replace(cmd, androidHome, "$ANDROID_HOME", -1)
	// }
	// if gomobilepath != "" {
	// 	cmd = strings.Replace(cmd, gomobilepath, "$GOMOBILE", -1)
	// }
	// if goroot := goEnv("GOROOT"); goroot != "" {
	// 	cmd = strings.Replace(cmd, goroot, "$GOROOT", -1)
	// }
	// if gopath := goEnv("GOPATH"); gopath != "" {
	// 	cmd = strings.Replace(cmd, gopath, "$GOPATH", -1)
	// }
	// if env := os.Getenv("HOME"); env != "" {
	// 	cmd = strings.Replace(cmd, env, "$HOME", -1)
	// }
	// if env := os.Getenv("HOMEPATH"); env != "" {
	// 	cmd = strings.Replace(cmd, env, "$HOMEPATH", -1)
	// }
	fmt.Fprint(xout, cmd)
}

// environ merges os.Environ and the given "key=value" pairs.
// If a key is in both os.Environ and kv, kv takes precedence.
func matchaEnviron(kv []string) []string {
	cur := os.Environ()
	new := make([]string, 0, len(cur)+len(kv))

	envs := make(map[string]string, len(cur))
	for _, ev := range cur {
		elem := strings.SplitN(ev, "=", 2)
		if len(elem) != 2 || elem[0] == "" {
			// pass the env var of unusual form untouched.
			// e.g. Windows may have env var names starting with "=".
			new = append(new, ev)
			continue
		}
		if goos == "windows" {
			elem[0] = strings.ToUpper(elem[0])
		}
		envs[elem[0]] = elem[1]
	}
	for _, ev := range kv {
		elem := strings.SplitN(ev, "=", 2)
		if len(elem) != 2 || elem[0] == "" {
			panic(fmt.Sprintf("malformed env var %q from input", ev))
		}
		if goos == "windows" {
			elem[0] = strings.ToUpper(elem[0])
		}
		envs[elem[0]] = elem[1]
	}
	for k, v := range envs {
		new = append(new, k+"="+v)
	}
	return new
}

func matchapkgdir(matchaGoMobilePath string, env []string) string {
	return matchaGoMobilePath + "/pkg_" + matchaGetenv(env, "GOOS") + "_" + matchaGetenv(env, "GOARCH")
}
