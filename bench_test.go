package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// BenchmarkHomeFrame measures a full headless frame build and paint of the
// loaded home screen, including the virtualized wallet list.
func BenchmarkHomeFrame(b *testing.B) {
	store := openTestStore(b)
	defer store.Close()
	app := NewApp(store)
	app.syncWork = true
	app.AttachStore(store)
	tt := ui.NewTester(app.view, 1120, 780)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}
