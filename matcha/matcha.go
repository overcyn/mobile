package matcha

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// don't forget to remove it!
func NewTmpDir(f *Flags) (string, error) {
	_gomobilepath, err := GoMobilePath()
	if err != nil {
		return "", err
	}

	// Make $GOPATH/pkg/work
	tmpdir := ""
	if f.ShouldRun() {
		tmpdir, err = ioutil.TempDir(_gomobilepath, "work-")
		if err != nil {
			return "", err
		}
	} else {
		tmpdir = filepath.Join(_gomobilepath, "work")
	}
	if f.ShouldPrint() {
		fmt.Fprintln(os.Stderr, "WORK="+tmpdir)
	}
	// defer func() {
	// 	if buildWork {
	// 		fmt.Printf("WORK=%s\n", tmpdir)
	// 		return
	// 	}
	// 	removeAll(tmpdir)
	// }()
	return tmpdir, err
}

func XcodeAvailable() bool {
	_, err := exec.LookPath("xcrun")
	return err == nil
}

func ArchClang(goarch string) string {
	switch goarch {
	case "arm":
		return "armv7"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	case "amd64":
		return "x86_64"
	default:
		panic(fmt.Sprintf("unknown GOARCH: %q", goarch))
	}
}

// Get clang path and clang flags (SDK Path).
func EnvClang(flags *Flags, sdkName string) (_clang, cflags string, err error) {
	cmd := exec.Command("xcrun", "--sdk", sdkName, "--find", "clang")
	var clang string
	if flags.ShouldPrint() {
		PrintCommand(cmd)
	}
	if flags.ShouldRun() {
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", "", fmt.Errorf("xcrun --find: %v\n%s", err, out)
		}
		clang = strings.TrimSpace(string(out))
	} else {
		clang = "clang-" + sdkName
	}

	cmd = exec.Command("xcrun", "--sdk", sdkName, "--show-sdk-path")
	var sdk string
	if flags.ShouldPrint() {
		PrintCommand(cmd)
	}
	if flags.ShouldRun() {
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", "", fmt.Errorf("xcrun --show-sdk-path: %v\n%s", err, out)
		}
		sdk = strings.TrimSpace(string(out))
	} else {
		sdk = sdkName
	}

	return clang, "-isysroot " + sdk, nil
}

func PrintCommand(cmd *exec.Cmd) {
	fmt.Println(cmd)
}

type Flags struct {
	BuildN    bool   // print commands but don't run
	BuildX    bool   // print commands
	BuildV    bool   // print package names
	BuildWork bool   // use working directory
	BuildO    string // output directory

	BuildA       bool   // -a
	BuildI       bool   // -i
	BuildGcflags string // -gcflags
	BuildLdflags string // -ldflags
	BuildTarget  string // -target
}

func (f *Flags) ShouldPrint() bool {
	return f.BuildN || f.BuildX
}

func (f *Flags) ShouldRun() bool {
	return !f.BuildN
}

func Init(flags *Flags) error {
	// Get GOPATH
	_gomobilepath, err := GoMobilePath()
	if err != nil {
		return err
	}

	// Delete $GOPATH/pkg/gomobile
	verpath := filepath.Join(_gomobilepath, "version")
	if flags.ShouldPrint() {
		fmt.Fprintln(os.Stderr, "GOMOBILE="+_gomobilepath)
	}
	RemoveAll(flags, _gomobilepath)

	// Make $GOPATH/pkg/gomobile
	if err := Mkdir(flags, _gomobilepath); err != nil {
		return err
	}

	// Make $GOPATH/pkg/work
	var tmpdir string
	if !flags.ShouldRun() {
		tmpdir = filepath.Join(_gomobilepath, "work")
	} else {
		var err error
		tmpdir, err = ioutil.TempDir(_gomobilepath, "work-")
		if err != nil {
			return err
		}
	}
	// if buildX || buildN {
	//  fmt.Fprintln(xout, "WORK="+tmpdir)
	// }
	defer func() {
		// if buildWork {
		//  fmt.Printf("WORK=%s\n", tmpdir)
		//  return
		// }
		RemoveAll(flags, tmpdir)
	}()

	// // Build NDK stuff?
	// if buildN {
	//  initNDK = "$NDK_PATH"
	//  initOpenAL = "$OPENAL_PATH"
	// } else {
	//  toolsDir := filepath.Join("prebuilt", archNDK(), "bin")
	//  // Try the ndk-bundle SDK package package, if installed.
	//  if initNDK == "" {
	//      if sdkHome := os.Getenv("ANDROID_HOME"); sdkHome != "" {
	//          path := filepath.Join(sdkHome, "ndk-bundle")
	//          if st, err := os.Stat(filepath.Join(path, toolsDir)); err == nil && st.IsDir() {
	//              initNDK = path
	//          }
	//      }
	//  }
	//  if initNDK != "" {
	//      var err error
	//      if initNDK, err = filepath.Abs(initNDK); err != nil {
	//          return err
	//      }
	//      // Check if the platform directory contains a known subdirectory.
	//      if _, err := os.Stat(filepath.Join(initNDK, toolsDir)); err != nil {
	//          if os.IsNotExist(err) {
	//              return fmt.Errorf("%q does not point to an Android NDK.", initNDK)
	//          }
	//          return err
	//      }
	//      ndkFile := filepath.Join(_gomobilepath, "android_ndk_root")
	//      if err := ioutil.WriteFile(ndkFile, []byte(initNDK), 0644); err != nil {
	//          return err
	//      }
	//  }
	//  if initOpenAL != "" {
	//      var err error
	//      if initOpenAL, err = filepath.Abs(initOpenAL); err != nil {
	//          return err
	//      }
	//  }
	// }
	// ndkRoot = initNDK
	// if err := matchaEnvInit(); err != nil {
	//  return err
	// }

	// // Install "golang.org/x/mobile/gl", "golang.org/x/mobile/app", "golang.org/x/mobile/exp/app/debug",
	// if runtime.GOOS == "darwin" {
	//  // Install common x/mobile packages for local development.
	//  // These are often slow to compile (due to cgo) and easy to forget.
	//  //
	//  // Limited to darwin for now as it is common for linux to
	//  // not have GLES installed.
	//  //
	//  // TODO: consider testing GLES installation and suggesting it here
	//  for _, pkg := range commonPkgs {
	//      if err := installPkg(pkg, nil); err != nil {
	//          return err
	//      }
	//  }
	// }

	// Install standard libraries for cross compilers.
	start := time.Now()
	// Ideally this would be -buildmode=c-shared.
	// https://golang.org/issue/13234.

	// androidArgs := []string{"-gcflags=-shared", "-ldflags=-shared"}
	// for _, arch := range archs {
	//  env := androidEnv[arch]
	//  if err := InstallPkg("std", env, _gomobilepath, androidArgs...); err != nil {
	//      return err
	//  }
	// }

	// Install iOS libraries
	var env []string

	if !XcodeAvailable() {
		return errors.New("Xcode not available")
	}

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

	// TODO(crawshaw): darwin/386 for the iOS simulator?
	if env, err = DarwinAmd64Env(flags); err != nil {
		return err
	}
	if err := InstallPkg(flags, tmpdir, "std", env, "-tags=ios"); err != nil {
		return err
	}

	// Write Go Version to $GOPATH/pkg/gomobile/version
	if flags.ShouldPrint() {
		Printcmd("go version > %s", verpath)
	}
	if flags.ShouldRun() {
		goversion, err := GoVersion()
		if err != nil {
			return nil
		}
		if err := ioutil.WriteFile(verpath, goversion, 0644); err != nil {
			return err
		}
	}
	if flags.BuildV {
		took := time.Since(start) / time.Second * time.Second
		fmt.Fprintf(os.Stderr, "\nDone, build took %s.\n", took)
	}
	return nil
}

func DarwinArmEnv(f *Flags) ([]string, error) {
	if !XcodeAvailable() {
		return nil, errors.New("Xcode not available")
	}
	clang, cflags, err := EnvClang(f, "iphoneos")
	if err != nil {
		return nil, err
	}
	return []string{
		"GOOS=darwin",
		"GOARCH=arm",
		"GOARM=7",
		"CC=" + clang,
		"CXX=" + clang,
		"CGO_CFLAGS=" + cflags + " -miphoneos-version-min=6.1 -arch " + ArchClang("arm"),
		"CGO_LDFLAGS=" + cflags + " -miphoneos-version-min=6.1 -arch " + ArchClang("arm"),
		"CGO_ENABLED=1",
	}, nil
}

func DarwinArm64Env(f *Flags) ([]string, error) {
	if !XcodeAvailable() {
		return nil, errors.New("Xcode not available")
	}
	clang, cflags, err := EnvClang(f, "iphoneos")
	if err != nil {
		return nil, err
	}
	return []string{
		"GOOS=darwin",
		"GOARCH=arm64",
		"CC=" + clang,
		"CXX=" + clang,
		"CGO_CFLAGS=" + cflags + " -miphoneos-version-min=6.1 -arch " + ArchClang("arm64"),
		"CGO_LDFLAGS=" + cflags + " -miphoneos-version-min=6.1 -arch " + ArchClang("arm64"),
		"CGO_ENABLED=1",
	}, nil
}

func Darwin386Env(f *Flags) ([]string, error) {
	if !XcodeAvailable() {
		return nil, errors.New("Xcode not available")
	}
	clang, cflags, err := EnvClang(f, "iphonesimulator")
	if err != nil {
		return nil, err
	}
	return []string{
		"GOOS=darwin",
		"GOARCH=386",
		"CC=" + clang,
		"CXX=" + clang,
		"CGO_CFLAGS=" + cflags + " -mios-simulator-version-min=6.1 -arch " + ArchClang("386"),
		"CGO_LDFLAGS=" + cflags + " -mios-simulator-version-min=6.1 -arch " + ArchClang("386"),
		"CGO_ENABLED=1",
	}, nil
}

func DarwinAmd64Env(f *Flags) ([]string, error) {
	if !XcodeAvailable() {
		return nil, errors.New("Xcode not available")
	}
	clang, cflags, err := EnvClang(f, "iphonesimulator")
	if err != nil {
		return nil, err
	}
	return []string{
		"GOOS=darwin",
		"GOARCH=amd64",
		"CC=" + clang,
		"CXX=" + clang,
		"CGO_CFLAGS=" + cflags + " -mios-simulator-version-min=6.1 -arch x86_64",
		"CGO_LDFLAGS=" + cflags + " -mios-simulator-version-min=6.1 -arch x86_64",
		"CGO_ENABLED=1",
	}, nil
}

func Getenv(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return kv[len(prefix):]
		}
	}
	return ""
}

// environ merges os.Environ and the given "key=value" pairs.
// If a key is in both os.Environ and kv, kv takes precedence.
func Environ(kv []string) []string {
	cur := os.Environ()
	new := make([]string, 0, len(cur)+len(kv))
	goos := runtime.GOOS

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

func Pkgdir(env []string) (string, error) {
	gomobilepath, err := GoMobilePath()
	if err != nil {
		return "", err
	}
	return gomobilepath + "/pkg_" + Getenv(env, "GOOS") + "_" + Getenv(env, "GOARCH"), nil
}

func Printcmd(format string, args ...interface{}) {
	cmd := fmt.Sprintf(format+"\n", args...)
	// if tmpdir != "" {
	//  cmd = strings.Replace(cmd, tmpdir, "$WORK", -1)
	// }
	// if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
	//  cmd = strings.Replace(cmd, androidHome, "$ANDROID_HOME", -1)
	// }
	// if gomobilepath != "" {
	//  cmd = strings.Replace(cmd, gomobilepath, "$GOMOBILE", -1)
	// }
	// if goroot := goEnv("GOROOT"); goroot != "" {
	//  cmd = strings.Replace(cmd, goroot, "$GOROOT", -1)
	// }
	// if gopath := goEnv("GOPATH"); gopath != "" {
	//  cmd = strings.Replace(cmd, gopath, "$GOPATH", -1)
	// }
	// if env := os.Getenv("HOME"); env != "" {
	//  cmd = strings.Replace(cmd, env, "$HOME", -1)
	// }
	// if env := os.Getenv("HOMEPATH"); env != "" {
	//  cmd = strings.Replace(cmd, env, "$HOMEPATH", -1)
	// }
	fmt.Fprint(os.Stderr, cmd)
}

func GoEnv(name string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}
	val, err := exec.Command("go", "env", name).Output()
	if err != nil {
		panic(err) // the Go tool was tested to work earlier
	}
	return strings.TrimSpace(string(val))
}

func RemoveAll(f *Flags, path string) error {
	if f.ShouldPrint() {
		Printcmd(`rm -r -f "%s"`, path)
	}
	if f.ShouldRun() {
		return os.RemoveAll(path)
	}
	return nil
}

func WriteFile(flags *Flags, filename string, generate func(io.Writer) error) error {
	if err := Mkdir(flags, filepath.Dir(filename)); err != nil {
		return err
	}
	if flags.ShouldPrint() {
		fmt.Fprintf(os.Stderr, "write %s\n", filename)
	}
	if flags.ShouldRun() {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}()
		return generate(f)
	}
	return generate(ioutil.Discard)
}

func CopyFile(f *Flags, dst, src string) error {
	if f.ShouldPrint() {
		Printcmd("cp %s %s", src, dst)
	}
	return WriteFile(f, dst, func(w io.Writer) error {
		if f.ShouldRun() {
			f, err := os.Open(src)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(w, f); err != nil {
				return fmt.Errorf("cp %s %s failed: %v", src, dst, err)
			}
		}
		return nil
	})
}

func RunCmd(f *Flags, tmpdir string, cmd *exec.Cmd) error {
	if f.ShouldPrint() {
		dir := ""
		if cmd.Dir != "" {
			dir = "PWD=" + cmd.Dir + " "
		}
		env := strings.Join(cmd.Env, " ")
		if env != "" {
			env += " "
		}
		Printcmd("%s%s%s", dir, env, strings.Join(cmd.Args, " "))
	}

	buf := new(bytes.Buffer)
	buf.WriteByte('\n')
	if f.BuildV {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = buf
		cmd.Stderr = buf
	}

	if f.BuildWork {
		if runtime.GOOS == "windows" {
			cmd.Env = append(cmd.Env, `TEMP=`+tmpdir)
			cmd.Env = append(cmd.Env, `TMP=`+tmpdir)
		} else {
			cmd.Env = append(cmd.Env, `TMPDIR=`+tmpdir)
		}
	}

	if !f.BuildN {
		cmd.Env = Environ(cmd.Env)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %v%s", strings.Join(cmd.Args, " "), err, buf)
		}
	}
	return nil
}

func InstallPkg(f *Flags, temporarydir string, pkg string, env []string, args ...string) error {
	pd, err := Pkgdir(env)
	if err != nil {
		return err
	}

	tOS, tArch := Getenv(env, "GOOS"), Getenv(env, "GOARCH")
	if tOS != "" && tArch != "" {
		if f.BuildV {
			fmt.Fprintf(os.Stderr, "\n# Installing %s for %s/%s.\n", pkg, tOS, tArch)
		}
		args = append(args, "-pkgdir="+pd)
	} else {
		if f.BuildV {
			fmt.Fprintf(os.Stderr, "\n# Installing %s.\n", pkg)
		}
	}

	cmd := exec.Command("go", "install")
	cmd.Args = append(cmd.Args, args...)
	if f.BuildV {
		cmd.Args = append(cmd.Args, "-v")
	}
	if f.BuildX {
		cmd.Args = append(cmd.Args, "-x")
	}
	if f.BuildWork {
		cmd.Args = append(cmd.Args, "-work")
	}
	cmd.Args = append(cmd.Args, pkg)
	cmd.Env = append([]string{}, env...)
	return RunCmd(f, temporarydir, cmd)
}

func GoVersion() ([]byte, error) {
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

func Mkdir(flags *Flags, dir string) error {
	if flags.ShouldPrint() {
		Printcmd("mkdir -p %s", dir)
	}
	if flags.ShouldRun() {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func Symlink(flags *Flags, src, dst string) error {
	if flags.ShouldPrint() {
		Printcmd("ln -s %s %s", src, dst)
	}
	if flags.ShouldRun() {
		// if goos == "windows" {
		// 	return doCopyAll(dst, src)
		// }
		return os.Symlink(src, dst)
	}
	return nil
}

func GoBuild(f *Flags, src string, env []string, ctx build.Context, tmpdir string, args ...string) error {
	return GoCmd(f, "build", []string{src}, env, ctx, tmpdir, args...)
}

func GoInstall(f *Flags, srcs []string, env []string, ctx build.Context, tmpdir string, args ...string) error {
	return GoCmd(f, "install", srcs, env, ctx, tmpdir, args...)
}

func GoMobilePath() (string, error) {
	gopaths := filepath.SplitList(GoEnv("GOPATH"))
	gomobilepath := ""
	for _, p := range gopaths {
		gomobilepath = filepath.Join(p, "pkg", "gomobile")
		if _, err := os.Stat(gomobilepath); err == nil {
			break
		}
	}
	if gomobilepath == "" {
		return "", fmt.Errorf("GOPATH is not set")
	}
	return gomobilepath, nil
}

func GoCmd(f *Flags, subcmd string, srcs []string, env []string, ctx build.Context, tmpdir string, args ...string) error {
	pd, err := Pkgdir(env)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", subcmd, "-pkgdir="+pd)
	if len(ctx.BuildTags) > 0 {
		cmd.Args = append(cmd.Args, "-tags", strings.Join(ctx.BuildTags, " "))
	}
	if f.BuildV {
		cmd.Args = append(cmd.Args, "-v")
	}
	if subcmd != "install" && f.BuildI {
		cmd.Args = append(cmd.Args, "-i")
	}
	if f.BuildX {
		cmd.Args = append(cmd.Args, "-x")
	}
	if f.BuildGcflags != "" {
		cmd.Args = append(cmd.Args, "-gcflags", f.BuildGcflags)
	}
	if f.BuildLdflags != "" {
		cmd.Args = append(cmd.Args, "-ldflags", f.BuildLdflags)
	}
	if f.BuildWork {
		cmd.Args = append(cmd.Args, "-work")
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Args = append(cmd.Args, srcs...)
	cmd.Env = append([]string{}, env...)
	return RunCmd(f, tmpdir, cmd)
}
