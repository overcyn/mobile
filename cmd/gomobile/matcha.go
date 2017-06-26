package main

import (
	"errors"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mobile/matcha"
)

var cmdMatcha = &command{
	run:   runMatcha,
	Name:  "matcha",
	Usage: "[-target android|ios] [-bootclasspath <path>] [-classpath <path>] [-o output] [build flags] [package]",
	Short: "build a library for Android and iOS",
	Long: `
Bind generates language bindings for the package named by the import
path, and compiles a library for the named target system.

The -target flag takes a target system name, either android (the
default) or ios.

For -target android, the bind command produces an AAR (Android ARchive)
file that archives the precompiled Java API stub classes, the compiled
shared libraries, and all asset files in the /assets subdirectory under
the package directory. The output is named '<package_name>.aar' by
default. This AAR file is commonly used for binary distribution of an
Android library project and most Android IDEs support AAR import. For
example, in Android Studio (1.2+), an AAR file can be imported using
the module import wizard (File > New > New Module > Import .JAR or
.AAR package), and setting it as a new dependency
(File > Project Structure > Dependencies).  This requires 'javac'
(version 1.7+) and Android SDK (API level 15 or newer) to build the
library for Android. The environment variable ANDROID_HOME must be set
to the path to Android SDK. The generated Java class is in the java
package 'go.<package_name>' unless -javapkg flag is specified.

By default, -target=android builds shared libraries for all supported
instruction sets (arm, arm64, 386, amd64). A subset of instruction sets
can be selected by specifying target type with the architecture name. E.g.,
-target=android/arm,android/386.

For -target ios, gomobile must be run on an OS X machine with Xcode
installed. Support is not complete. The generated Objective-C types
are prefixed with 'Go' unless the -prefix flag is provided.

For -target android, the -bootclasspath and -classpath flags are used to
control the bootstrap classpath and the classpath for Go wrappers to Java
classes.

The -v flag provides verbose output, including the list of packages built.

The build flags -a, -n, -x, -gcflags, -ldflags, -tags, and -work
are shared with the build command. For documentation, see 'go help build'.
`,
}

func runMatcha(cmd *command) error {
	cleanup, err := buildEnvInit()
	if err != nil {
		return err
	}
	defer cleanup()

	args := cmd.flag.Args()

	targetOS, targetArchs, err := parseBuildTarget(buildTarget)
	if err != nil {
		return fmt.Errorf(`invalid -target=%q: %v`, buildTarget, err)
	}

	ctx.GOARCH = "arm"
	ctx.GOOS = targetOS

	if ctx.GOOS == "darwin" {
		ctx.BuildTags = append(ctx.BuildTags, "ios")
	}

	if bindJavaPkg != "" && ctx.GOOS != "android" {
		return fmt.Errorf("-javapkg is supported only for android target")
	}
	if bindPrefix != "" && ctx.GOOS != "darwin" {
		return fmt.Errorf("-prefix is supported only for ios target")
	}

	if ctx.GOOS == "android" && ndkRoot == "" {
		return errors.New("no Android NDK path is set. Please run gomobile init with the ndk-bundle installed through the Android SDK manager or with the -ndk flag set.")
	}

	var pkgs []*build.Package
	switch len(args) {
	case 0:
		pkgs = make([]*build.Package, 1)
		pkgs[0], err = ctx.ImportDir(cwd, build.ImportComment)
	default:
		pkgs, err = importPackages(args)
	}
	if err != nil {
		return err
	}

	// check if any of the package is main
	for _, pkg := range pkgs {
		if pkg.Name == "main" {
			return fmt.Errorf("binding 'main' package (%s) is not supported", pkg.ImportComment)
		}
	}

	switch targetOS {
	case "android":
		return goAndroidBind(pkgs, targetArchs)
	case "darwin":
		// TODO: use targetArchs?
		return matchaIOSBind(pkgs, cmd)
	default:
		return fmt.Errorf(`invalid -target=%q`, buildTarget)
	}
}

func matchaIOSBind(pkgs []*build.Package, command *command) error {
	flags := &matcha.Flags{
		BuildN:       buildN,
		BuildX:       buildX,
		BuildV:       buildV,
		BuildWork:    buildWork,
		BuildO:       buildO,
		BuildA:       buildA,
		BuildI:       buildI,
		BuildGcflags: buildGcflags,
		BuildLdflags: buildLdflags,
		BuildTarget:  buildTarget,
	}

	name := "matcha"
	title := "Matcha"
	tempDir := tmpdir
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
	if err := matcha.Mkdir(flags, bridgeDir); err != nil {
		return err
	}

	// Create the "main" go package, that references the other go packages
	mainPath := filepath.Join(tempDir, "src", "iosbin", "main.go")
	err := matcha.WriteFile(flags, mainPath, func(w io.Writer) error {
		blah := command.flag.Args()[0]
		format := fmt.Sprintf(string(iosBindFile), blah)
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
	if err := matcha.CopyFile(flags, filepath.Join(bridgeDir, "matchaobjc.h"), filepath.Join(objcPkg.Dir, "matchaobjc.h.support")); err != nil {
		return err
	}
	if err := matcha.CopyFile(flags, filepath.Join(bridgeDir, "matchaobjc.m"), filepath.Join(objcPkg.Dir, "matchaobjc.m.support")); err != nil {
		return err
	}
	if err := matcha.CopyFile(flags, filepath.Join(bridgeDir, "matchaobjc.go"), filepath.Join(objcPkg.Dir, "matchaobjc.go.support")); err != nil {
		return err
	}
	if err := matcha.CopyFile(flags, filepath.Join(bridgeDir, "matchago.h"), filepath.Join(objcPkg.Dir, "matchago.h.support")); err != nil {
		return err
	}
	if err := matcha.CopyFile(flags, filepath.Join(bridgeDir, "matchago.m"), filepath.Join(objcPkg.Dir, "matchago.m.support")); err != nil {
		return err
	}
	if err := matcha.CopyFile(flags, filepath.Join(bridgeDir, "matchago.go"), filepath.Join(objcPkg.Dir, "matchago.go.support")); err != nil {
		return err
	}

	// Build static framework output directory.
	if err := matcha.RemoveAll(flags, frameworkDir); err != nil {
		return err
	}

	// Build framework directory structure.
	headersDir := filepath.Join(frameworkDir, "Versions", "A", "Headers")
	resourcesDir := filepath.Join(frameworkDir, "Versions", "A", "Resources")
	modulesDir := filepath.Join(frameworkDir, "Versions", "A", "Modules")
	binaryPath := filepath.Join(frameworkDir, "Versions", "A", title)
	if err := matcha.Mkdir(flags, headersDir); err != nil {
		return err
	}
	if err := matcha.Mkdir(flags, resourcesDir); err != nil {
		return err
	}
	if err := matcha.Mkdir(flags, modulesDir); err != nil {
		return err
	}
	if err := matcha.Symlink(flags, "A", filepath.Join(frameworkDir, "Versions", "Current")); err != nil {
		return err
	}
	if err := matcha.Symlink(flags, filepath.Join("Versions", "Current", "Headers"), filepath.Join(frameworkDir, "Headers")); err != nil {
		return err
	}
	if err := matcha.Symlink(flags, filepath.Join("Versions", "Current", "Resources"), filepath.Join(frameworkDir, "Resources")); err != nil {
		return err
	}
	if err := matcha.Symlink(flags, filepath.Join("Versions", "Current", "Modules"), filepath.Join(frameworkDir, "Modules")); err != nil {
		return err
	}
	if err := matcha.Symlink(flags, filepath.Join("Versions", "Current", title), filepath.Join(frameworkDir, title)); err != nil {
		return err
	}

	// Copy in headers.
	if err = matcha.CopyFile(flags, filepath.Join(headersDir, "matchaobjc.h"), filepath.Join(bridgeDir, "matchaobjc.h")); err != nil {
		return err
	}
	if err = matcha.CopyFile(flags, filepath.Join(headersDir, "matchago.h"), filepath.Join(bridgeDir, "matchago.h")); err != nil {
		return err
	}

	// Copy in resources.
	if err := ioutil.WriteFile(filepath.Join(resourcesDir, "Info.plist"), []byte(iosBindInfoPlist), 0666); err != nil {
		return err
	}

	// Write modulemap.
	var mmVals = struct {
		Module  string
		Headers []string
	}{
		Module:  title,
		Headers: []string{"matchaobjc.h", "matchago.h"},
	}
	err = matcha.WriteFile(flags, filepath.Join(modulesDir, "module.modulemap"), func(w io.Writer) error {
		return iosModuleMapTmpl.Execute(w, mmVals)
	})
	if err != nil {
		return err
	}

	// Build platform binaries concurrently.
	matchaDarwinArmEnv, err := matcha.DarwinArmEnv(flags)
	if err != nil {
		return err
	}

	matchaDarwinArm64Env, err := matcha.DarwinArm64Env(flags)
	if err != nil {
		return err
	}

	matchaDarwinAmd64Env, err := matcha.DarwinAmd64Env(flags)
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
			arch := getenv(env, "GOARCH")
			env = append(env, "GOPATH="+genDir+string(filepath.ListSeparator)+os.Getenv("GOPATH"))
			path := filepath.Join(tempDir, name+"-"+arch+".a")
			err := matcha.GoBuild(flags, mainPath, env, ctx, tmpdir, "-buildmode=c-archive", "-o", path)
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
		cmd.Args = append(cmd.Args, "-arch", matcha.ArchClang(i.arch), i.path)
	}
	cmd.Args = append(cmd.Args, "-o", binaryPath)
	return matcha.RunCmd(flags, tempDir, cmd)
}
