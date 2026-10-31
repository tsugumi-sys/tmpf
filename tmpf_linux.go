package tmpf

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func create(dir string) (*os.File, error) {
	return createWithOTmpfile(dir, unix.Open)
}

func createWithOTmpfile(dir string, openFile func(string, int, uint32) (int, error)) (*os.File, error) {
	var fd int
	var err error
	for {
		// O_EXCL prevents later linking of the anonymous file into the filesystem.
		fd, err = openFile(dir, unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC|unix.O_EXCL, 0o600)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	if err == nil {
		return os.NewFile(uintptr(fd), filepath.Join(dir, "tmpf")), nil
	}

	// EISDIR and ENOENT are documented results on kernels without O_TMPFILE.
	// ENOENT can also mean a missing directory; CreateTemp will report that
	// error too. EINVAL covers environments that reject the O_TMPFILE flags.
	if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EISDIR) ||
		errors.Is(err, unix.ENOENT) || errors.Is(err, unix.EINVAL) {
		f, err := os.CreateTemp(dir, "tmpf-*")
		if err != nil {
			return nil, err
		}
		return unlink(f)
	}
	return nil, &os.PathError{Op: "open", Path: dir, Err: err}
}
