# tmpf

Temporary files that do not need `os.Remove`.

`tmpf.Create` returns a readable, writable, seekable `*os.File` without leaving a
file name in the directory. Close the file when finished; the OS reclaims its
resources after the last reference is closed.

Supports **Linux and macOS**, with **Go 1.26 or later**.

## Install

```sh
go get github.com/tsugumi-sys/tmpf
```

## Usage

```go
package main

import (
	"fmt"
	"io"
	"log"

	"github.com/tsugumi-sys/tmpf"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	f, err := tmpf.Create("") // Empty dir uses os.TempDir().
	if err != nil {
		return err
	}
	defer f.Close() // No os.Remove is needed.

	if _, err := f.WriteString("hello\n"); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	contents, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	fmt.Print(string(contents))
	return nil
}
```

Pass an existing directory to control the filesystem used:

```go
f, err := tmpf.Create("/path/to/scratch")
```

## Behavior

| | `os.CreateTemp` | `tmpf.Create` |
| --- | --- | --- |
| File name after successful creation | Present | Absent |
| Caller must remove the file | Yes | No |
| Caller must close the file | Yes | Yes |
| Read, write, seek | Yes | Yes |
| Reopen by name | Yes | Not supported |

On Linux, tmpf first tries `O_TMPFILE` with `O_CLOEXEC` and `O_EXCL`: the file
never has a directory entry, is not accidentally inherited across exec, and
cannot later be linked into the filesystem. If the environment does not support
this approach, tmpf falls back to `os.CreateTemp` followed immediately by
unlinking the name. macOS always uses the unlink approach. Permission, capacity,
and other operational errors are returned rather than hidden by a fallback.

Files are created with permissions `0o600` before umask. A missing directory is
an error; tmpf does not create directories. `File.Name()` is for diagnostics and
does not provide a path for reopening or passing to an external command.

## Cleanup guarantees

- A successful call leaves no directory entry for the created file.
- Callers must close the returned file; cleanup does not depend on garbage
  collection or finalizers.
- If a process exits after creation succeeds, its file descriptors are closed
  by the OS. Duplicated descriptors, mappings, or references deliberately passed
  to another process can keep the file's resources alive.
- The unlink approach briefly creates a named file. A process killed between
  creation and unlinking can leave that file behind.
- If unlinking fails, tmpf closes the opened file and returns an error. The named
  file may remain. The no-removal guarantee applies to successful calls.

## Development

```sh
go test -race ./...
go vet ./...
```

CI runs tests on Linux and macOS using Go 1.26 and the latest stable Go release.

Commits to `main` must have a signature verified by GitHub. Configure your signing
key using [GitHub's signing guide](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits),
then enable signing for this repository:

```sh
git config --local commit.gpgsign true
```

## License

MIT. See [LICENSE](LICENSE).
