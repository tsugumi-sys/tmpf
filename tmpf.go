//go:build linux || darwin

package tmpf

import "os"

func Create(dir string) (*os.File, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	return create(dir)
}
