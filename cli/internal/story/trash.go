package story

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// TrashFunc is what cleanup uses; tests replace it.
var TrashFunc = Trash

// Trash moves a file or folder to the user's trash (shell.trashItem).
// macOS: ~/.Trash. Linux: the XDG trash with a .trashinfo record.
func Trash(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var dir, infoDir string
	switch runtime.GOOS {
	case "darwin":
		dir = filepath.Join(home, ".Trash")
	case "linux":
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "share")
		}
		dir = filepath.Join(base, "Trash", "files")
		infoDir = filepath.Join(base, "Trash", "info")
	default:
		return fmt.Errorf("moving to the trash is not supported on %s", runtime.GOOS)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := filepath.Base(abs)
	dst := filepath.Join(dir, name)
	for i := 2; exists(dst); i++ {
		ext := filepath.Ext(name)
		dst = filepath.Join(dir, fmt.Sprintf("%s %d%s", strings.TrimSuffix(name, ext), i, ext))
	}
	if infoDir != "" {
		os.MkdirAll(infoDir, 0o700)
		info := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n", (&url.URL{Path: abs}).EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
		os.WriteFile(filepath.Join(infoDir, filepath.Base(dst)+".trashinfo"), []byte(info), 0o600)
	}
	return moveFile(abs, dst)
}

// moveFile renames, falling back to copy+remove across devices.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := CopyTree(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// Exists reports whether a path exists.
func Exists(p string) bool { return exists(p) }

// CopyFile copies one file, creating parent folders.
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// CopyTree copies a file or a directory recursively.
func CopyTree(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return CopyFile(src, dst)
	}
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return CopyFile(p, target)
	})
}
