package tmpf

import "os"

func create(dir string) (*os.File, error) {
	f, err := os.CreateTemp(dir, "tmpf-*")
	if err != nil {
		return nil, err
	}
	return unlink(f)
}
