//go:build linux || darwin

// Package tmpf creates temporary files without requiring callers to remove them.
// Callers must still close each file when it is no longer needed.
//
// On Linux, tmpf prefers O_TMPFILE and falls back to creating and immediately
// unlinking a named file. On macOS, it always uses the unlink approach.
package tmpf

import (
	"errors"
	"os"
)

// Create creates an empty, readable, writable, seekable temporary file in dir.
// If dir is empty, Create uses [os.TempDir]. The directory must already exist.
// The file is created with mode 0o600, before applying the process's umask.
//
// On success, no directory entry remains for the file. The caller must close
// the returned file, but does not need to remove it. The file's Name is for
// diagnostics only and is not a path that can be used to reopen the file.
//
// The unlink approach briefly creates a named file. If the process terminates
// between creation and unlinking, or unlinking fails, that file may remain.
// On error, Create returns a nil file and closes any file it opened.
func Create(dir string) (*os.File, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	return create(dir)
}

func createUnlinked(dir string) (*os.File, error) {
	f, err := os.CreateTemp(dir, "tmpf-*")
	if err != nil {
		return nil, err
	}
	return unlink(f)
}

// unlink transfers ownership of f to the caller only after removing its name.
func unlink(f *os.File) (*os.File, error) {
	if err := os.Remove(f.Name()); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return f, nil
}
