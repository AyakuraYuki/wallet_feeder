package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// TestMain points the SQLite plugin at the native library in resources when
// one is checked out beside the app. On a machine without it the plugin
// downloads its verified copy into the user cache as usual.
func TestMain(m *testing.M) {
	if os.Getenv("MYGO_SQLITE_LIBRARY") == "" {
		name := bundledSQLiteName(runtime.GOOS, runtime.GOARCH)
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

func TestSingleInstanceLockHelperProcess(t *testing.T) {
	if os.Getenv("WALLETFEEDER_INSTANCE_HELPER") == "" {
		t.Skip("single-instance helper process")
	}
	mygo.App.SetName(os.Getenv("WALLETFEEDER_INSTANCE_NAME"))
	if !mygo.App.RequestSingleInstanceLock() {
		t.Fatal("helper could not acquire the first instance lock")
	}
	fmt.Println("locked")
	_, _ = os.Stdin.Read(make([]byte, 1))
}

func TestSingleInstanceRejectsSecondProcess(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("desktop process locks are not available on js")
	}
	oldName := mygo.App.Name()
	name := fmt.Sprintf("WalletFeeder-single-instance-test-%d", os.Getpid())
	defer mygo.App.SetName(oldName)
	mygo.App.SetName(name)

	cmd := exec.Command(os.Args[0], "-test.run=^TestSingleInstanceLockHelperProcess$")
	cmd.Env = append(os.Environ(), "WALLETFEEDER_INSTANCE_HELPER=1", "WALLETFEEDER_INSTANCE_NAME="+name)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = stdin.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err == nil && strings.TrimSpace(line) != "locked" {
			err = fmt.Errorf("unexpected helper output %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("helper did not acquire the lock: %v; stderr=%s", err, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the first instance")
	}
	if mygo.App.RequestSingleInstanceLock() {
		t.Fatal("second process acquired the same instance lock")
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("helper exited with error: %v; stderr=%s", err, stderr.String())
		}
		waited = true
	case <-time.After(10 * time.Second):
		t.Fatal("second instance did not hand control back to the first instance")
	}
}

func TestSecureUserDataDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not available on Windows")
	}
	dir := filepath.Join(t.TempDir(), "private", "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := secureUserDataDir(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("data directory mode = %04o, want 0700", got)
	}
}

func TestBundledSQLiteNameChecksArchitecture(t *testing.T) {
	if got := bundledSQLiteName("darwin", "arm64"); got != "libmygo-sqlite3.dylib" {
		t.Fatalf("darwin/arm64 library = %q", got)
	}
	if got := bundledSQLiteName("darwin", "amd64"); got != "" {
		t.Fatalf("darwin/amd64 should discover its own library, got %q", got)
	}
}
