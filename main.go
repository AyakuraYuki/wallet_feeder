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

	app := NewApp(nil)
	if dir := os.Getenv("WALLETFEEDER_DATA_DIR"); dir != "" {
		mygo.App.SetPath(mygo.PathUserData, dir)
	}
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
			MinHeight:       720,
			StateKey:        "main",
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
		app.start(filepath.Join(dir, "walletfeeder.db"))
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// useBundledSQLite points the plugin at the library shipped in the app's
// resources. MyGo normally discovers it on its own; naming it explicitly
// also makes development builds under a restrictive sandbox use the local
// copy instead of trying to download one into the user cache.
func useBundledSQLite() {
	if os.Getenv("MYGO_SQLITE_LIBRARY") != "" {
		return
	}
	name := map[string]string{
		"darwin":  "libmygo-sqlite3.dylib",
		"linux":   "libmygo-sqlite3.so",
		"windows": "mygo-sqlite3.dll",
	}[runtime.GOOS]
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
