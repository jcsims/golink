package main

import (
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func exitOnError(errorString string, err error) {
	if err != nil {
		slog.Error(errorString, "error", err)
		os.Exit(1)
	}
}

// Take the full path of a dotfile to be symlinked, and return a path relative
// from the home directory for the symlink to live.
func convertToHomePath(dotsPath, homeDir, dotsFilePath string) string {
	homePath, err := filepath.Rel(dotsPath, dotsFilePath)
	exitOnError("Unable to get relative dots file path", err)

	homePath = strings.TrimSuffix(homePath, ".symlink")
	homePath = filepath.Join(homeDir, homePath)

	return homePath
}

// Make sure the directory is present on disk.
func ensureDir(homePath string) {
	base := filepath.Dir(homePath)
	err := os.MkdirAll(base, 0755)
	exitOnError("Unable to create target directory for symlink", err)
}

// A file that we're trying to symlink is already there. There are a few cases:
// - It's symlinked to the file we're trying to symlink it to. Happy path, all
// is good.
// - Either a proper file already exists, or a symlink pointing somewhere else.
// Print a failure message and continue.
func handleExistingFile(homePath, dotsPath string) {
	info, err := os.Lstat(homePath)
	if err != nil {
		slog.Error("Unable to stat existing file", "error", err)

		return
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		linkedTarget, err := os.Readlink(homePath)
		if err != nil {
			slog.Error("Unable to `readlink` on existing symlink", "error", err)

			return
		}

		if linkedTarget != dotsPath {
			slog.Warn("Existing file points to different target, not symlinking!", "homePath", homePath, "linkedTarget", linkedTarget)
		}
		// If it points to the current dots file, then nothing to do!
	} else {
		slog.Debug("Existing file at path, not symlinking!", "path", homePath)
	}

}

func symlinkFile(homePath, dotsFilePath string) {
	err := os.Symlink(dotsFilePath, homePath)
	if os.IsExist(err) {
		handleExistingFile(homePath, dotsFilePath)
	} else if err != nil {
		slog.Error("Unable to create symlink at homePath due to error", "homePath", homePath, "error", err)
	} else {
		slog.Info("Symlinked source file", "dotsPath", dotsFilePath, "homePath", homePath)
	}
}

func walk(dotsPath, homeDir string) func(string, fs.DirEntry, error) error {
	return func(dotsFilePath string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("Got an error visiting file", "file", dotsFilePath)
			return nil
		}
		if filepath.Ext(dotsFilePath) == ".symlink" {
			homeFilePath := convertToHomePath(dotsPath, homeDir, dotsFilePath)
			ensureDir(homeFilePath)
			symlinkFile(homeFilePath, dotsFilePath)
		}

		return nil
	}
}

func initLogging(verbose bool) {
	var logLevel slog.Level
	if verbose {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelWarn
	}

	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(h))

}

// TODO: Implement dry-run
func main() {
	verbose := flag.Bool("v", false, "Turn on verbose logging")
	dotfiles := flag.String("dotfiles", ".dotfiles", "Path to dotfiles to link. If relative, assumed to be relative to user's home directory.")
	flag.Parse()
	initLogging(*verbose)

	homeDir, err := os.UserHomeDir()
	exitOnError("Unable to get users homedir", err)

	var dotsPath string

	if filepath.IsAbs(*dotfiles) {
		dotsPath = *dotfiles
	} else {
		dotsPath = filepath.Join(homeDir, *dotfiles)
	}

	err = filepath.WalkDir(dotsPath, walk(dotsPath, homeDir))
	exitOnError("Unable to walk dotfiles directory", err)
}
