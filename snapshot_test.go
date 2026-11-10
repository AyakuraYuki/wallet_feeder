package main

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSnapshots is a development aid: run with WF_SNAPSHOT=1 to render the
// screens to artifacts/ for visual review. It is skipped in normal test
// runs.
func TestSnapshots(t *testing.T) {
	if os.Getenv("WF_SNAPSHOT") == "" {
		t.Skip("set WF_SNAPSHOT=1 to write UI snapshots")
	}
	store := openTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 18, 30, 0, 0, time.Local)
	store.now = func() time.Time { return now }

	// A small lived-in ledger.
	wallets := []struct {
		name    string
		initial int64
	}{
		{"日常钱包", 3_240_00},
		{"招行储蓄卡", 3_800_00},
		{"支付宝", 260_00},
		{"现金", 60_00},
	}
	for _, w := range wallets {
		if _, err := store.CreateWallet(ctx, w.name, w.initial); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := store.Snapshot(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int64{}
	for _, w := range snap.Wallets {
		byName[w.Name] = w.ID
	}
	entries := []struct {
		day      int
		wallet   string
		kind     Kind
		category string
		cents    int64
		note     string
	}{
		{2, "日常钱包", KindExpense, "餐饮", 42_50, "楼下面馆"},
		{3, "支付宝", KindExpense, "交通", 12_00, "地铁"},
		{5, "招行储蓄卡", KindIncome, "工资", 12_000_00, "11 月工资"},
		{6, "日常钱包", KindExpense, "购物", 328_00, "冬袜和护手霜"},
		{7, "招行储蓄卡", KindExpense, "居住", 4_200_00, "房租"},
		{8, "日常钱包", KindExpense, "餐饮", 68_00, "和朋友吃火锅"},
		{9, "支付宝", KindExpense, "娱乐", 45_00, "电影票"},
		{9, "现金", KindExpense, "餐饮", 26_00, ""},
		{10, "现金", KindExpense, "餐饮", 56_00, "早饭和咖啡"},
		{10, "日常钱包", KindExpense, "交通", 18_00, "打车"},
	}
	for _, e := range entries {
		at := time.Date(2026, 11, e.day, 9+e.day%9, 15, 0, 0, time.Local)
		if _, err := store.AddEntry(ctx, NewEntry{
			WalletID: byName[e.wallet], Kind: e.kind, Category: e.category,
			AmountCents: e.cents, Note: e.note, OccurredAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetBudget(ctx, 8_000_00); err != nil {
		t.Fatal(err)
	}

	app := NewApp(store)
	app.syncWork = true
	app.now = func() time.Time { return now }
	app.ledgerMonth = monthStart(now)
	app.AttachStore(store)

	dir := filepath.Join("docs", "screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(app.view, 1120, 780)
	save := func(name string) {
		img := tt.Image()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	save("home.png")
	app.page = pageLedger
	tt.Frame()
	save("ledger.png")
	app.page = pageWallets
	tt.Frame()
	save("wallets.png")
	app.page = pageHome
	app.openBudget()
	tt.Frame()
	save("budget-modal.png")

	app.closeModal()
	app.reaction = &feedReaction{kind: KindExpense, walletName: "日常钱包", amountCents: 3650}
	tt.Frame()
	save("home-reaction.png")

	app.reaction = nil
	tt.SetSize(1000, 760)
	tt.Frame()
	save("home-compact.png")

	tt.SetSize(1120, 780)
	tt.SetDark(true)
	tt.Frame()
	save("home-dark.png")
}
