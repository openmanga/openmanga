package story

import (
	"os"
	"testing"
)

// Tests never touch the real user-data folder.
func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "sb-userdata-")
	os.Setenv("SB_USER_DATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
