//go:build linux || darwin

package tmpf

import (
	"errors"
	"os"
)

func unlink(f *os.File) (*os.File, error) {
	if err := os.Remove(f.Name()); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return f, nil
}
