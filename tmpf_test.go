//go:build linux || darwin

package tmpf

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCreate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f, err := Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	assertTemporaryFile(t, f)

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Stat after Close: got %v, want os.ErrClosed", err)
	}
	assertEmptyDir(t, dir)
}

func TestCreateDefaultDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	f, err := Create("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	assertTemporaryFile(t, f)
	assertEmptyDir(t, dir)

	// A missing TMPDIR must fail rather than silently using another directory.
	t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
	other, err := Create("")
	if other != nil {
		_ = other.Close()
		t.Fatal("Create ignored the missing TMPDIR")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v, want os.ErrNotExist", err)
	}
}

func TestCreateInvalidDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regularFile := filepath.Join(dir, "file")
	if err := os.WriteFile(regularFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		dir  string
		want error
	}{
		{"missing", filepath.Join(dir, "missing", "nested"), os.ErrNotExist},
		{"regular file", regularFile, syscall.ENOTDIR},
		{"nul byte", "\x00", syscall.EINVAL},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Create(tt.dir)
			if f != nil {
				_ = f.Close()
				t.Fatal("Create returned a file on error")
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCreatePermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can bypass directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	f, err := Create(dir)
	if f != nil {
		_ = f.Close()
		t.Fatal("Create returned a file without directory write permission")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("got %v, want os.ErrPermission", err)
	}
	assertEmptyDir(t, dir)
}

func TestUnlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "tmpf-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	result, err := unlink(f)
	if err != nil {
		t.Fatal(err)
	}
	assertTemporaryFile(t, result)
	assertEmptyDir(t, dir)
}

func TestUnlinkFailureClosesFile(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "tmpf-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	// Removing the name first deterministically makes the next unlink fail.
	if err := os.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
	result, err := unlink(f)
	if result != nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unlink returned (%v, %v), want (nil, os.ErrNotExist)", result, err)
	}
	if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("file left open after unlink failure: %v", err)
	}
}

func TestUnlinkFailurePreservesCloseError(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "tmpf-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
	result, err := unlink(f)
	if result != nil || !errors.Is(err, os.ErrNotExist) || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("unlink returned (%v, %v), want both remove and close errors", result, err)
	}
}

func TestCreateConcurrent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	t.Cleanup(func() { assertEmptyDir(t, dir) })
	for i := range 16 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			f, err := Create(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			assertTemporaryFile(t, f)
		})
	}
}

func TestCreateProcessExit(t *testing.T) {
	for _, mode := range []string{"exit", "kill"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCreateProcessHelper$", "-test.timeout=10s")
			cmd.Env = append(os.Environ(), "TMPF_TEST_PROCESS="+mode, "TMPF_TEST_DIR="+dir)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			}()
			ready, err := bufio.NewReader(stdout).ReadString('\n')
			if err != nil || ready != "ready\n" {
				t.Fatalf("child did not create a file: ready=%q, err=%v", ready, err)
			}
			assertEmptyDir(t, dir)
			if mode == "kill" {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				var exitErr *exec.ExitError
				if err := cmd.Wait(); !errors.As(err, &exitErr) {
					t.Fatalf("killed child returned %v, want exec.ExitError", err)
				}
			} else if err := cmd.Wait(); err != nil {
				t.Fatal(err)
			}
			assertEmptyDir(t, dir)
		})
	}
}

func TestCreateProcessHelper(t *testing.T) {
	mode := os.Getenv("TMPF_TEST_PROCESS")
	if mode == "" {
		return
	}
	f, err := Create(os.Getenv("TMPF_TEST_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("temporary contents"); err != nil {
		t.Fatal(err)
	}
	fmt.Println("ready")
	if mode == "kill" {
		var b [1]byte
		_, _ = os.Stdin.Read(b[:])
	}
	runtime.KeepAlive(f)
	// Deliberately exit without Close or deferred cleanup.
	os.Exit(0)
}

func assertTemporaryFile(t *testing.T, f *os.File) {
	t.Helper()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("want an empty regular file, got mode=%v size=%d", info.Mode(), info.Size())
	}
	if info.Mode().Perm() & ^os.FileMode(0o600) != 0 {
		t.Fatalf("permissions %o exceed 0o600", info.Mode().Perm())
	}
	if n := info.Sys().(*syscall.Stat_t).Nlink; n != 0 {
		t.Fatalf("file still has %d directory links", n)
	}
	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFD, 0)
	if err != nil {
		t.Fatal(err)
	}
	if flags&unix.FD_CLOEXEC == 0 {
		t.Fatal("file descriptor is missing close-on-exec")
	}
	if offset, err := f.Seek(0, io.SeekCurrent); err != nil || offset != 0 {
		t.Fatalf("initial offset: got (%d, %v), want (0, nil)", offset, err)
	}
	const contents = "temporary contents\n"
	if _, err := f.WriteString(contents); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != contents {
		t.Fatalf("got %q, want %q", got, contents)
	}
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary directory still contains %v", entries)
	}
}
