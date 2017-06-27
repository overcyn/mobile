package matcha

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

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
	if !XcodeAvailable() {
		return "", "", errors.New("Xcode not available")
	}

	cmd := exec.Command("xcrun", "--sdk", sdkName, "--find", "clang")
	var clang string
	if flags.ShouldPrint() {
		fmt.Fprintln(os.Stderr, cmd)
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
		fmt.Fprintln(os.Stderr, cmd)
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

func DarwinArmEnv(f *Flags) ([]string, error) {
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
	fmt.Fprintln(os.Stderr, cmd)
}

// Returns the enviromental variable for name.
func GoEnv(name string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}
	val, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(val))
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

func GoVersion(f *Flags) ([]byte, error) {
	cmd := exec.Command("go", "version")
	if f.ShouldPrint() {
		fmt.Fprintln(os.Stderr, cmd)
	}
	if f.ShouldRun() {
		goVer, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("'go version' failed: %v, %s", err, goVer)
		}
		if bytes.HasPrefix(goVer, []byte("go version go1.4")) || bytes.HasPrefix(goVer, []byte("go version go1.5")) || bytes.HasPrefix(goVer, []byte("go version go1.6")) {
			return nil, errors.New("Go 1.7 or newer is required")
		}
		return goVer, nil
	}
	return []byte("go version goX.X.X"), nil
}

func GoBuild(f *Flags, src string, env []string, ctx build.Context, tmpdir string, args ...string) error {
	return GoCmd(f, "build", []string{src}, env, ctx, tmpdir, args...)
}

func GoInstall(f *Flags, srcs []string, env []string, ctx build.Context, tmpdir string, args ...string) error {
	return GoCmd(f, "install", srcs, env, ctx, tmpdir, args...)
}

// $GOPATH/pkg/gomobile
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
		if len(gopaths) == 0 {
			return "", fmt.Errorf("$GOPATH does not exist")
		} else {
			return filepath.Join(gopaths[0], "pkg", "gomobile"), nil
		}
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

var iosModuleMapTmpl = template.Must(template.New("iosmmap").Parse(`framework module "{{.Module}}" {
    // header "ref.h"
{{range .Headers}}    header "{{.}}"
{{end}}
    export *
}`))

func WriteModuleMap(flags *Flags, filename string, title string) error {
	// Write modulemap.
	var mmVals = struct {
		Module  string
		Headers []string
	}{
		Module:  title,
		Headers: []string{"matchaobjc.h", "matchago.h"},
	}
	err := WriteFile(flags, filename, func(w io.Writer) error {
		return iosModuleMapTmpl.Execute(w, mmVals)
	})
	if err != nil {
		return err
	}
	return nil
}

const IosBindInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0">
      <dict>
      </dict>
    </plist>
`

func IOSBind(flags *Flags, pkgs []*build.Package, firstArg string, tempDir string, ctx build.Context) error {
	name := "matcha"
	title := "Matcha"
	genDir := filepath.Join(tempDir, "gen")
	frameworkDir := flags.BuildO
	if frameworkDir != "" && !strings.HasSuffix(frameworkDir, ".framework") {
		return fmt.Errorf("static framework name %q missing .framework suffix", frameworkDir)
	}
	if frameworkDir == "" {
		frameworkDir = title + ".framework"
	}

	// Build the "matcha/bridge" dir
	bridgeDir := filepath.Join(genDir, "src", "github.com", "overcyn", "matchabridge")
	if err := Mkdir(flags, bridgeDir); err != nil {
		return err
	}

	// Create the "main" go package, that references the other go packages
	mainPath := filepath.Join(tempDir, "src", "iosbin", "main.go")
	err := WriteFile(flags, mainPath, func(w io.Writer) error {
		format := fmt.Sprintf(string(iosBindFile), firstArg)
		_, err := w.Write([]byte(format))
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to create the binding package for iOS: %v", err)
	}

	// Get the supporting files
	objcPkg, err := ctx.Import("golang.org/x/mobile/bind/objc", "", build.FindOnly)
	if err != nil {
		return err
	}
	if err := CopyFile(flags, filepath.Join(bridgeDir, "matchaobjc.h"), filepath.Join(objcPkg.Dir, "matchaobjc.h.support")); err != nil {
		return err
	}
	if err := CopyFile(flags, filepath.Join(bridgeDir, "matchaobjc.m"), filepath.Join(objcPkg.Dir, "matchaobjc.m.support")); err != nil {
		return err
	}
	if err := CopyFile(flags, filepath.Join(bridgeDir, "matchaobjc.go"), filepath.Join(objcPkg.Dir, "matchaobjc.go.support")); err != nil {
		return err
	}
	if err := CopyFile(flags, filepath.Join(bridgeDir, "matchago.h"), filepath.Join(objcPkg.Dir, "matchago.h.support")); err != nil {
		return err
	}
	if err := CopyFile(flags, filepath.Join(bridgeDir, "matchago.m"), filepath.Join(objcPkg.Dir, "matchago.m.support")); err != nil {
		return err
	}
	if err := CopyFile(flags, filepath.Join(bridgeDir, "matchago.go"), filepath.Join(objcPkg.Dir, "matchago.go.support")); err != nil {
		return err
	}

	// Build static framework output directory.
	if err := RemoveAll(flags, frameworkDir); err != nil {
		return err
	}

	// Build framework directory structure.
	headersDir := filepath.Join(frameworkDir, "Versions", "A", "Headers")
	resourcesDir := filepath.Join(frameworkDir, "Versions", "A", "Resources")
	modulesDir := filepath.Join(frameworkDir, "Versions", "A", "Modules")
	binaryPath := filepath.Join(frameworkDir, "Versions", "A", title)
	if err := Mkdir(flags, headersDir); err != nil {
		return err
	}
	if err := Mkdir(flags, resourcesDir); err != nil {
		return err
	}
	if err := Mkdir(flags, modulesDir); err != nil {
		return err
	}
	if err := Symlink(flags, "A", filepath.Join(frameworkDir, "Versions", "Current")); err != nil {
		return err
	}
	if err := Symlink(flags, filepath.Join("Versions", "Current", "Headers"), filepath.Join(frameworkDir, "Headers")); err != nil {
		return err
	}
	if err := Symlink(flags, filepath.Join("Versions", "Current", "Resources"), filepath.Join(frameworkDir, "Resources")); err != nil {
		return err
	}
	if err := Symlink(flags, filepath.Join("Versions", "Current", "Modules"), filepath.Join(frameworkDir, "Modules")); err != nil {
		return err
	}
	if err := Symlink(flags, filepath.Join("Versions", "Current", title), filepath.Join(frameworkDir, title)); err != nil {
		return err
	}

	// Copy in headers.
	if err = CopyFile(flags, filepath.Join(headersDir, "matchaobjc.h"), filepath.Join(bridgeDir, "matchaobjc.h")); err != nil {
		return err
	}
	if err = CopyFile(flags, filepath.Join(headersDir, "matchago.h"), filepath.Join(bridgeDir, "matchago.h")); err != nil {
		return err
	}

	// Copy in resources.
	if err := ioutil.WriteFile(filepath.Join(resourcesDir, "Info.plist"), []byte(IosBindInfoPlist), 0666); err != nil {
		return err
	}

	// Write modulemap.
	err = WriteModuleMap(flags, filepath.Join(modulesDir, "module.modulemap"), title)
	if err != nil {
		return err
	}

	// Build platform binaries concurrently.
	matchaDarwinArmEnv, err := DarwinArmEnv(flags)
	if err != nil {
		return err
	}

	matchaDarwinArm64Env, err := DarwinArm64Env(flags)
	if err != nil {
		return err
	}

	matchaDarwinAmd64Env, err := DarwinAmd64Env(flags)
	if err != nil {
		return err
	}

	type archPath struct {
		arch string
		path string
		err  error
	}
	archChan := make(chan archPath)
	for _, i := range [][]string{matchaDarwinArmEnv, matchaDarwinArm64Env, matchaDarwinAmd64Env} {
		go func(env []string) {
			arch := Getenv(env, "GOARCH")
			env = append(env, "GOPATH="+genDir+string(filepath.ListSeparator)+os.Getenv("GOPATH"))
			path := filepath.Join(tempDir, name+"-"+arch+".a")
			err := GoBuild(flags, mainPath, env, ctx, tempDir, "-buildmode=c-archive", "-o", path)
			archChan <- archPath{arch, path, err}
		}(i)
	}
	archs := []archPath{}
	for i := 0; i < 3; i++ {
		arch := <-archChan
		if arch.err != nil {
			return arch.err
		}
		archs = append(archs, arch)
	}

	// Lipo to build fat binary.
	cmd := exec.Command("xcrun", "lipo", "-create")
	for _, i := range archs {
		cmd.Args = append(cmd.Args, "-arch", ArchClang(i.arch), i.path)
	}
	cmd.Args = append(cmd.Args, "-o", binaryPath)
	return RunCmd(flags, tempDir, cmd)
}

func ImportPackages(args []string, ctx build.Context, cwd string) ([]*build.Package, error) {
	pkgs := make([]*build.Package, len(args))
	for i, a := range args {
		a = path.Clean(a)
		var err error
		if pkgs[i], err = ctx.Import(a, cwd, build.ImportComment); err != nil {
			return nil, fmt.Errorf("package %q: %v", a, err)
		}
	}
	return pkgs, nil
}

func ParseBuildTarget(buildTarget string) (os string, archs []string, _ error) {
	if buildTarget == "" {
		return "", nil, fmt.Errorf(`invalid target ""`)
	}

	all := false
	archNames := []string{}
	for i, p := range strings.Split(buildTarget, ",") {
		osarch := strings.SplitN(p, "/", 2) // len(osarch) > 0
		if osarch[0] != "android" && osarch[0] != "ios" {
			return "", nil, fmt.Errorf(`unsupported os`)
		}

		if i == 0 {
			os = osarch[0]
		}

		if os != osarch[0] {
			return "", nil, fmt.Errorf(`cannot target different OSes`)
		}

		if len(osarch) == 1 {
			all = true
		} else {
			archNames = append(archNames, osarch[1])
		}
	}

	// verify all archs are supported one while deduping.
	var supported []string
	switch os {
	case "ios":
		supported = []string{"arm", "arm64", "amd64"}
	case "android":
		supported = []string{"arm", "arm64", "386", "amd64"}
	}

	isSupported := func(arch string) bool {
		for _, a := range supported {
			if a == arch {
				return true
			}
		}
		return false
	}

	seen := map[string]bool{}
	for _, arch := range archNames {
		if _, ok := seen[arch]; ok {
			continue
		}
		if !isSupported(arch) {
			return "", nil, fmt.Errorf(`unsupported arch: %q`, arch)
		}

		seen[arch] = true
		archs = append(archs, arch)
	}

	targetOS := os
	if os == "ios" {
		targetOS = "darwin"
	}
	if all {
		return targetOS, supported, nil
	}
	return targetOS, archs, nil
}

var iosBindFile = []byte(`
package main

import (
    _ "github.com/overcyn/matchabridge"
    _ "%s"
)

import "C"

func main() {}
`)

var iosBindHeaderTmpl = template.Must(template.New("ios.h").Parse(`
// Objective-C API for talking to the following Go packages
//
{{range .pkgs}}//   {{.ImportPath}}
{{end}}//
// File is generated by gomobile bind. Do not edit.
#ifndef __{{.title}}_FRAMEWORK_H__
#define __{{.title}}_FRAMEWORK_H__

{{range .bases}}#include "{{.}}.objc.h"
{{end}}
#endif
`))
