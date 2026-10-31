# tmpf

Temporary files with cleanup on close.

`tmpf.Create` returns a readable, writable, seekable `*os.File`. Close the file
when finished; the OS reclaims its resources after the last reference is closed.

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
	defer f.Close() // Close the file when finished.

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

See [Architecture](docs/arch.md) for file behavior and cleanup details.

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
