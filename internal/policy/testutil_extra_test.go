package policy

import (
	"os"
)

func testWorkingDir(t interface{ Helper() }) string {
	_ = t
	wd, _ := os.Getwd()
	return wd
}

func testChdir(t interface{ Helper() }, dir string) error {
	_ = t
	return os.Chdir(dir)
}
