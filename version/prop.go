// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package version

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"tailscale.com/tailcfg"
)

// IsMobile reports whether this is a mobile client build.
func IsMobile() bool {
	return runtime.GOOS == "android" || runtime.GOOS == "ios"
}

// OS returns runtime.GOOS, except instead of returning "darwin" it returns
// "iOS" or "macOS".
func OS() string {
	// If you're wondering why we have this function that just returns
	// runtime.GOOS written differently: in the old days, Go reported
	// GOOS=darwin for both iOS and macOS, so we needed this function to
	// differentiate them. Then a later Go release added GOOS=ios as a separate
	// platform, but by then the "iOS" and "macOS" values we'd picked, with that
	// exact capitalization, were already baked into databases.
	if runtime.GOOS == "ios" {
		return "iOS"
	}
	if runtime.GOOS == "darwin" {
		return "macOS"
	}
	return runtime.GOOS
}

var (
	macFlavorOnce  sync.Once
	isMacSysExt    bool
	isMacSandboxed bool
)

func initMacFlavor() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	isMacSysExt = filepath.Base(exe) == "io.tailscale.ipn.macsys.network-extension"
	isMacSandboxed = isMacSysExt || strings.HasSuffix(exe, "/Contents/MacOS/Tailscale") || strings.HasSuffix(exe, "/Contents/MacOS/IPNExtension")
}

// IsSandboxedMacOS reports whether this process is a sandboxed macOS
// process (either the app or the extension). It is true for the Mac App Store
// and macsys (System Extension) version on macOS, and false for
// tailscaled-on-macOS.
func IsSandboxedMacOS() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	macFlavorOnce.Do(initMacFlavor)
	return isMacSandboxed
}

// IsMacSysExt whether this binary is from the standalone "System
// Extension" (a.k.a. "macsys") version of Tailscale for macOS.
func IsMacSysExt() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	macFlavorOnce.Do(initMacFlavor)
	return isMacSysExt
}

var (
	winFlavorOnce sync.Once
	isWindowsGUI  bool
)

func initWinFlavor() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	isWindowsGUI = strings.EqualFold(exe, "tailscale-ipn.exe") || strings.EqualFold(exe, "tailscale-ipn")
}

// IsWindowsGUI reports whether the current process is the Windows GUI.
func IsWindowsGUI() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	exe, _ := os.Executable()
	exe = filepath.Base(exe)
	return strings.EqualFold(exe, "tailscale-ipn.exe") || strings.EqualFold(exe, "tailscale-ipn")
}

var (
	isUnstableOnce  sync.Once
	isUnstableBuild bool
)

// IsUnstableBuild reports whether this is an unstable build.
// That is, whether its minor version number is odd.
func IsUnstableBuild() bool {
	isUnstableOnce.Do(initUnstable)
	return isUnstableBuild
}

func initUnstable() {
	_, rest, ok := strings.Cut(Short, ".")
	if !ok {
		return
	}
	minorStr, _, ok := strings.Cut(rest, ".")
	if !ok {
		return
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return
	}
	isUnstableBuild = minor%2 == 1
}

// Meta is a JSON-serializable type that contains all the version
// information.
type Meta struct {
	// MajorMinorPatch is the "major.minor.patch" version string, without
	// any hyphenated suffix.
	MajorMinorPatch string `json:"majorMinorPatch"`

	// IsDev is whether Short contains a -dev suffix. This is whether the build
	// is a development build (as opposed to an official stable or unstable
	// build stamped in the usual ways). If you just run "go install" or "go
	// build" on a dev branch, this will be true.
	IsDev bool `json:"isDev,omitempty"`

	// Short is MajorMinorPatch but optionally adding "-dev" or "-devYYYYMMDD"
	// for dev builds, depending on how it was build.
	Short string `json:"short"`

	// Long is the full version string, including git commit hash(es) as the
	// suffix.
	Long string `json:"long"`

	// UnstableBranch is whether the build is from an unstable (development)
	// branch. That is, it reports whether the minor version is odd.
	UnstableBranch bool `json:"unstableBranch,omitempty"`

	// GitCommit, if non-empty, is the git commit of the
	// github.com/tailscale/tailscale repository at which Tailscale was
	// built. Its format is the one returned by `git describe --always
	// --exclude "*" --dirty --abbrev=200`.
	GitCommit string `json:"gitCommit,omitempty"`

	// GitDirty is whether Go stamped the binary as having dirty version
	// control changes in the working directory (debug.ReadBuildInfo
	// setting "vcs.modified" was true).
	GitDirty bool `json:"gitDirty,omitempty"`

	// ExtraGitCommit, if non-empty, is the git commit of a "supplemental"
	// repository at which Tailscale was built. Its format is the same as
	// gitCommit.
	//
	// ExtraGitCommit is used to track the source revision when the main
	// Tailscale repository is integrated into and built from another
	// repository (for example, Tailscale's proprietary code, or the
	// Android OSS repository). Together, GitCommit and ExtraGitCommit
	// exactly describe what repositories and commits were used in a
	// build.
	ExtraGitCommit string `json:"extraGitCommit,omitempty"`

	// DaemonLong is the version number from the tailscaled
	// daemon, if requested.
	DaemonLong string `json:"daemonLong,omitempty"`

	// Cap is the current Tailscale capability version. It's a monotonically
	// incrementing integer that's incremented whenever a new capability is
	// added.
	Cap int `json:"cap"`
}

// GetMeta returns version metadata about the current build.
func GetMeta() Meta {
	return Meta{
		MajorMinorPatch: majorMinorPatch,
		Short:           Short,
		Long:            Long,
		GitCommit:       GitCommit,
		GitDirty:        GitDirty,
		ExtraGitCommit:  ExtraGitCommit,
		IsDev:           strings.Contains(Short, "-dev"), // TODO(bradfitz): could make a bool for this in init
		UnstableBranch:  IsUnstableBuild(),
		Cap:             int(tailcfg.CurrentCapabilityVersion),
	}
}
