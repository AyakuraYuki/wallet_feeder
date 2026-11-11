package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	mygo.App.SetName("WalletFeeder")
	mygo.App.SetVersion("0.1.0")

	if dir := os.Getenv("WALLETFEEDER_DATA_DIR"); dir != "" {
		mygo.App.SetPath(mygo.PathUserData, dir)
	}
	// One process owns the ledger. Besides avoiding competing UI instances,
	// this prevents another connection from interleaving writes between the
	// reads in a snapshot or changing a balance during calibration.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	app := NewApp(nil)
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleWindowMenu},
		}))

		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "钱包饲养员",
			Width:           1120,
			Height:          780,
			UseContentSize:  true,
			MinWidth:        1000,
			MinHeight:       760,
			StateKey:        "main",
			TitleBarStyle:   mygo.TitleBarHiddenInset,
			TitleBarHeight:  58,
			BackgroundColor: "light-dark(#F6F0E7, #1C1916)",
			Content:         ui.View(app.view),
		})
		app.dispatch = func(fn func()) { win.Update(fn) }

		mygo.App.OnQuit(app.close)

		useBundledSQLite()
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			app.fatalErr = "找不到用于保存账本的用户目录：" + err.Error()
			return
		}
		if err := secureUserDataDir(dir); err != nil {
			app.fatalErr = "无法保护账本目录：" + err.Error()
			return
		}
		app.start(filepath.Join(dir, "walletfeeder.db"))

	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func secureUserDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// MkdirAll preserves an existing broad mode, so tighten it as well.
	return os.Chmod(dir, 0o700)
}

// Only the macOS arm64 SQLite library is checked into this repository.
// Other architectures must keep the plugin's matching-library discovery.
func bundledSQLiteName(goos, goarch string) string {
	if goos == "darwin" && goarch != "arm64" {
		return ""
	}
	return map[string]string{
		"darwin":  "libmygo-sqlite3.dylib",
		"linux":   "libmygo-sqlite3.so",
		"windows": "mygo-sqlite3.dll",
	}[goos]
}

// useBundledSQLite points the plugin at the library shipped in the app's
// resources. MyGo normally discovers it on its own; naming it explicitly
// also makes development builds under a restrictive sandbox use the local
// copy instead of trying to download one into the user cache.
func useBundledSQLite() {
	if os.Getenv("MYGO_SQLITE_LIBRARY") != "" {
		return
	}
	name := bundledSQLiteName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		return
	}
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return
	}
	path := filepath.Join(dir, name)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		_ = os.Setenv("MYGO_SQLITE_LIBRARY", path)
	}
}
