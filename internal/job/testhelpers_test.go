package job

import "os"

func mkdirAll(p string) error {
	return os.MkdirAll(p, 0o700)
}
