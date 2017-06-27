package matcha

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
)

func Bind(flags *Flags, args []string) error {
	// Make $WORK.
	tempdir, err := NewTmpDir(flags, "")
	if err != nil {
		return err
	}
	defer RemoveAll(flags, tempdir)

	// Get $GOPATH/pkg/gomobile.
	gomobilepath, err := GoMobilePath()
	if err != nil {
		return err
	}

	// Get toolchain version.
	installedVersion, err := ReadFile(flags, filepath.Join(gomobilepath, "version"))
	if err != nil {
		return errors.New("toolchain partially installed, run `gomobile init`")
	}

	// Get go version.
	goVersion, err := GoVersion(flags)
	if err != nil {
		return err
	}

	// Check toolchain matches go version.
	if !bytes.Equal(installedVersion, goVersion) {
		return errors.New("toolchain out of date, run `gomobile init`")
	}

	targetOS, _, err := ParseBuildTarget(flags.BuildTarget)
	if err != nil {
		return fmt.Errorf(`invalid -target=%q: %v`, flags.BuildTarget, err)
	}

	ctx := build.Default
	ctx.GOARCH = "arm"
	ctx.GOOS = targetOS
	if ctx.GOOS == "darwin" {
		ctx.BuildTags = append(ctx.BuildTags, "ios")
	}

	// Get current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Get packages to be built.
	pkgs := []*build.Package{}
	if len(args) == 0 {
		pkg, err := ctx.ImportDir(cwd, build.ImportComment)
		if err != nil {
			return err
		}
		pkgs = append(pkgs, pkg)
	} else {
		for _, a := range args {
			a = path.Clean(a)
			pkg, err := ctx.Import(a, cwd, build.ImportComment)
			if err != nil {
				return fmt.Errorf("package %q: %v", a, err)
			}
			pkgs = append(pkgs, pkg)
		}
	}

	// Check if any of the package is main.
	for _, pkg := range pkgs {
		if pkg.Name == "main" {
			return fmt.Errorf("binding 'main' package (%s) is not supported", pkg.ImportComment)
		}
	}

	switch targetOS {
	case "android":
		return errors.New("Android unsupporetd")
	case "darwin":
		// TODO: use targetArchs?
		if err = IOSBind(flags, pkgs, args[0], tempdir, ctx); err != nil {
			return err
		}
	default:
		return fmt.Errorf(`invalid -target=%q`, flags.BuildTarget)
	}
	return nil
}
