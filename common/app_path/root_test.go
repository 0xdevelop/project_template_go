package app_path

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExecutableRootAcrossCWD(t *testing.T) {
	if os.Getenv("APP_PATH_TEST_CHILD") == "1" {
		r, e := ExecutableDir()
		if e != nil {
			t.Fatal(e)
		}
		p, e := Resolve(r, "data/resource")
		if e != nil || p != filepath.Join(r, "data/resource") {
			t.Fatal(p, e)
		}
		if r != os.Getenv("APP_PATH_EXPECT") {
			t.Fatal(r)
		}
		return
	}
	root, e := ExecutableDir()
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		cmd := exec.Command(os.Args[0], "-test.run=^TestExecutableRootAcrossCWD$")
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "APP_PATH_TEST_CHILD=1", "APP_PATH_EXPECT="+root)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatal(e, string(out))
		}
	}
}
