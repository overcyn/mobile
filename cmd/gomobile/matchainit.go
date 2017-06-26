package main

import (
	"golang.org/x/mobile/matcha"
)

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

func runInitMatcha(cmd *command) error {
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
	return matcha.Init(flags)
}
