package tmpf

import "os"

func create(dir string) (*os.File, error) {
	return createUnlinked(dir)
}
