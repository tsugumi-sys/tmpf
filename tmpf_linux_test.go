package tmpf

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCreateLinuxFallback(t *testing.T) {
	t.Parallel()
	for _, errno := range []unix.Errno{unix.EOPNOTSUPP, unix.EISDIR, unix.ENOENT, unix.EINVAL} {
		t.Run(errno.Error(), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			f, err := createWithOTmpfile(dir, func(string, int, uint32) (int, error) {
				return -1, errno
			})
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			assertTemporaryFile(t, f)
			assertEmptyDir(t, dir)
		})
	}
}

func TestCreateLinuxPreservesOtherErrors(t *testing.T) {
	t.Parallel()
	for _, errno := range []unix.Errno{
		unix.EACCES, unix.EPERM, unix.ENOSPC, unix.EMFILE, unix.ENFILE, unix.EROFS, unix.EIO,
	} {
		t.Run(errno.Error(), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			f, err := createWithOTmpfile(dir, func(string, int, uint32) (int, error) {
				return -1, errno
			})
			if f != nil {
				_ = f.Close()
				t.Fatal("unexpected fallback for an operational error")
			}
			if !errors.Is(err, errno) {
				t.Fatalf("got %v, want %v", err, errno)
			}
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || pathErr.Path != dir {
				t.Fatalf("error does not identify the requested directory: %v", err)
			}
			assertEmptyDir(t, dir)
		})
	}
}

func TestCreateLinuxRetriesInterruptedOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	calls := 0
	f, err := createWithOTmpfile(dir, func(path string, flags int, mode uint32) (int, error) {
		calls++
		if calls == 1 {
			return -1, unix.EINTR
		}
		return unix.Open(path, flags, mode)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if calls != 2 {
		t.Fatalf("open called %d times, want 2", calls)
	}
	assertTemporaryFile(t, f)
	assertEmptyDir(t, dir)
}

func TestCreateLinuxAnonymous(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var openErr error
	f, err := createWithOTmpfile(dir, func(path string, flags int, mode uint32) (int, error) {
		fd, err := unix.Open(path, flags, mode)
		openErr = err
		return fd, err
	})
	if f != nil {
		defer f.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	if openErr != nil {
		t.Skipf("test filesystem does not support O_TMPFILE: %v", openErr)
	}
	assertTemporaryFile(t, f)
	// O_EXCL must prevent giving the anonymous file a permanent name.
	// Use /proc/self/fd so this check does not require CAP_DAC_READ_SEARCH.
	procFD := filepath.Join("/proc/self/fd", strconv.FormatUint(uint64(f.Fd()), 10))
	if _, err := os.Stat(procFD); err != nil {
		t.Skipf("/proc/self/fd is unavailable: %v", err)
	}
	if err := unix.Linkat(unix.AT_FDCWD, procFD, unix.AT_FDCWD, filepath.Join(dir, "linked"), unix.AT_SYMLINK_FOLLOW); !errors.Is(err, unix.ENOENT) {
		t.Fatalf("linking an O_EXCL temporary file returned %v, want ENOENT", err)
	}
	assertEmptyDir(t, dir)
}
