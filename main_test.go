package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain points the SQLite plugin at the native library in resources when
// one is checked out beside the app. On a machine without it the plugin
// downloads its verified copy into the user cache as usual.
func TestMain(m *testing.M) {
	if os.Getenv("MYGO_SQLITE_LIBRARY") == "" {
		name := map[string]string{
			"darwin":  "libmygo-sqlite3.dylib",
			"linux":   "libmygo-sqlite3.so",
			"windows": "mygo-sqlite3.dll",
		}[runtime.GOOS]
		if name != "" {
			if abs, err := filepath.Abs(filepath.Join("resources", name)); err == nil {
				if info, err := os.Stat(abs); err == nil && info.Mode().IsRegular() {
					_ = os.Setenv("MYGO_SQLITE_LIBRARY", abs)
				}
			}
		}
	}
	os.Exit(m.Run())
}
