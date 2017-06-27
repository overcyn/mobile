package main

import (
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

	return matcha.Bind(flags, cmd.flag.Args())
	// tempdir, err := matcha.NewBindTmpDir(flags)
	// if err != nil {
	// 	return err
	// }
	// defer func() {
	// 	if flags.BuildWork {
	// 		fmt.Printf("WORK=%s\n", tempdir)
	// 		return
	// 	}
	// 	matcha.RemoveAll(flags, tempdir)
	// }()

	// if flags.ShouldRun() {
	// 	_gomobilepath, err := matcha.GoMobilePath()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	goVersion, err := matcha.GoVersion()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	verpath := filepath.Join(_gomobilepath, "version")
	// 	installedVersion, err := ioutil.ReadFile(verpath)
	// 	if err != nil {
	// 		return errors.New("toolchain partially installed, run `gomobile init`")
	// 	}
	// 	if !bytes.Equal(installedVersion, goVersion) {
	// 		return errors.New("toolchain out of date, run `gomobile init`")
	// 	}
	// }

	// workingdir, err := os.Getwd()
	// if err != nil {
	// 	return err
	// }

	// // cleanup, err := buildEnvInit()
	// // if err != nil {
	// // 	return err
	// // }
	// // defer cleanup()

	// args := cmd.flag.Args()

	// targetOS, targetArchs, err := matcha.ParseBuildTarget(flags.BuildTarget)
	// if err != nil {
	// 	return fmt.Errorf(`invalid -target=%q: %v`, flags.BuildTarget, err)
	// }

	// _ctx := build.Default
	// _ctx.GOARCH = "arm"
	// _ctx.GOOS = targetOS

	// if _ctx.GOOS == "darwin" {
	// 	_ctx.BuildTags = append(_ctx.BuildTags, "ios")
	// }

	// // if bindJavaPkg != "" && _ctx.GOOS != "android" {
	// // 	return fmt.Errorf("-javapkg is supported only for android target")
	// // }
	// // if bindPrefix != "" && _ctx.GOOS != "darwin" {
	// // 	return fmt.Errorf("-prefix is supported only for ios target")
	// // }

	// // if _ctx.GOOS == "android" && ndkRoot == "" {
	// // 	return errors.New("no Android NDK path is set. Please run gomobile init with the ndk-bundle installed through the Android SDK manager or with the -ndk flag set.")
	// // }

	// var pkgs []*build.Package
	// switch len(args) {
	// case 0:
	// 	pkgs = make([]*build.Package, 1)
	// 	pkgs[0], err = _ctx.ImportDir(workingdir, build.ImportComment)
	// default:
	// 	pkgs, err = matcha.ImportPackages(args, _ctx, workingdir)
	// }
	// if err != nil {
	// 	return err
	// }

	// // check if any of the package is main
	// for _, pkg := range pkgs {
	// 	if pkg.Name == "main" {
	// 		return fmt.Errorf("binding 'main' package (%s) is not supported", pkg.ImportComment)
	// 	}
	// }

	// switch targetOS {
	// case "android":
	// 	return goAndroidBind(pkgs, targetArchs)
	// case "darwin":
	// 	// TODO: use targetArchs?
	// 	return matcha.IOSBind(flags, pkgs, args[0], tempdir, _ctx)
	// default:
	// 	return fmt.Errorf(`invalid -target=%q`, buildTarget)
	// }
}
