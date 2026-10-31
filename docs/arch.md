# Architecture

## Behavior

Files are anonymous when `Create` succeeds. They support read, write, and seek,
with permissions `0o600` before umask.

| | `os.CreateTemp` | `tmpf.Create` |
| --- | --- | --- |
| Access | File path and `*os.File` | `*os.File` |
| Caller cleanup | `Close` and `Remove` | `Close` |

`Create` uses an existing directory. An empty `dir` selects `os.TempDir()`.
`File.Name()` is a diagnostic label.

## Creation

- Linux uses `O_TMPFILE | O_RDWR | O_CLOEXEC | O_EXCL`.
- macOS and the Linux fallback use `os.CreateTemp`, followed immediately by
  `os.Remove` while keeping the file open.

`O_CLOEXEC` closes the descriptor on exec. `O_EXCL` keeps the file anonymous.
Linux falls back on `EOPNOTSUPP`, `EISDIR`, `ENOENT`, or `EINVAL`.
Other errors propagate to the caller.

## Cleanup

The caller closes the file. The OS reclaims its resources after the last
reference closes. Duplicated descriptors, mappings, and references shared with
other processes extend that lifetime. Process exit closes that process's
descriptors.

The unlink approach briefly creates a named file. Process termination during
that interval can leave the file behind. An unlink error closes the opened file
and returns the error; the named file may remain.
