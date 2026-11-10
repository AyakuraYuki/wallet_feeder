package main

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func newUITestApp(t *testing.T) (*App, *SQLiteStore) {
	t.Helper()
	store := openTestStore(t)
	a := NewApp(store)
	a.syncWork = true
	a.now = func() time.Time { return testNow }
	a.ledgerMonth = monthStart(testNow)
	a.AttachStore(store)
	t.Cleanup(func() { a.close() })
	return a, store
}

func testerFor(a *App) *ui.Tester {
	return ui.NewTester(a.view, 1120, 780)
}

func TestEmptyHomeShowsGuidanceAndZeroes(t *testing.T) {
	a, _ := newUITestApp(t)
	tt := testerFor(a)
	if !tt.HasText("还没投喂过，从第一笔开始吧") {
		t.Fatalf("empty guidance missing; texts: %v", tt.Texts())
	}
	if !tt.HasText("¥0.00") {
		t.Fatalf("zero totals missing; texts: %v", tt.Texts())
	}
	if tt.HasText("这个月喂去了哪里") {
		t.Fatal("empty category chart should be hidden")
	}
}

func TestInvalidAmountDisablesFeed(t *testing.T) {
	a, _ := newUITestApp(t)
	tt := testerFor(a)
	if err := tt.Click("金额"); err != nil {
		t.Fatal(err)
	}
	tt.Type("0")
	feed, ok := tt.AccessByRole(ui.RoleButton, "投喂钱包")
	if !ok {
		t.Fatalf("feed button missing; texts: %v", tt.Texts())
	}
	if !feed.Disabled {
		t.Fatal("feed button should be disabled for a zero amount")
	}
	if !tt.HasText("金额要大于 0") {
		t.Fatalf("amount hint missing; texts: %v", tt.Texts())
	}
	before := a.snap.CurrentStats.ExpenseCents
	_ = tt.Click("投喂钱包") // a disabled control takes no activation
	if a.snap.CurrentStats.ExpenseCents != before {
		t.Fatal("invalid amount wrote a record")
	}
}

func TestQuickFeedWithEnter(t *testing.T) {
	a, store := newUITestApp(t)
	tt := testerFor(a)
	if err := tt.Click("金额"); err != nil {
		t.Fatal(err)
	}
	tt.Type("36.50")
	if err := tt.Click("分类：餐饮"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("钱包：默认钱包"); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("金额") {
		t.Fatalf("amount input did not regain focus after choosing chips; focused texts: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)

	snap, err := store.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentStats.ExpenseCents != 3650 || len(snap.Ledger) != 1 {
		t.Fatalf("after Enter: stats=%+v ledger=%d", snap.CurrentStats, len(snap.Ledger))
	}
	if snap.Wallets[0].BalanceCents != -3650 {
		t.Fatalf("wallet balance = %d, want -3650", snap.Wallets[0].BalanceCents)
	}
	if a.form.amount != "" || a.form.category != "" || a.form.note != "" {
		t.Fatalf("form not cleared: %+v", a.form)
	}
	if a.reaction == nil {
		t.Fatal("feeding reaction missing")
	}
	if !tt.HasText("被喂了一口") {
		t.Fatalf("reaction copy missing; texts: %v", tt.Texts())
	}
	if !tt.Focused("金额") {
		t.Fatal("amount input should be focused for the next record")
	}
	tt.Advance(reactionDuration + 50*time.Millisecond)
	if a.reaction != nil {
		t.Fatal("reaction did not settle after its 340ms")
	}
}

func TestMissingCategoryShakesInsteadOfSaving(t *testing.T) {
	a, store := newUITestApp(t)
	tt := testerFor(a)
	if err := tt.Click("金额"); err != nil {
		t.Fatal(err)
	}
	tt.Type("12")
	if err := tt.Click("投喂钱包"); err != nil {
		t.Fatal(err)
	}
	if a.shakeCategoryUntil.IsZero() {
		t.Fatal("category selector should shake when missing")
	}
	if !tt.HasText("先挑一个分类") {
		t.Fatalf("missing category hint; texts: %v", tt.Texts())
	}
	snap, err := store.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 0 {
		t.Fatalf("record saved without a category: %+v", snap.Ledger)
	}
	// The form stays typeable: choosing the category now should be enough.
	if err := tt.Click("分类：交通"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("投喂钱包"); err != nil {
		t.Fatal(err)
	}
	snap, err = store.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 1 || snap.CurrentStats.ExpenseCents != 1200 {
		t.Fatalf("record after choosing a category = %+v", snap.CurrentStats)
	}
}

func TestIncomePathUsesIncomeCategoriesAndAddsBalance(t *testing.T) {
	a, store := newUITestApp(t)
	tt := testerFor(a)
	if err := tt.Click("金额"); err != nil {
		t.Fatal(err)
	}
	tt.Type("800")
	if err := tt.Click("记录收入"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("工资") || tt.HasText("餐饮") {
		t.Fatalf("income categories did not replace expense ones; texts: %v", tt.Texts())
	}
	if err := tt.Click("分类：红包"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("钱包：默认钱包"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("投喂钱包"); err != nil {
		t.Fatal(err)
	}
	snap, err := store.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentStats.IncomeCents != 80000 || snap.CurrentStats.ExpenseCents != 0 {
		t.Fatalf("income stats = %+v", snap.CurrentStats)
	}
	if snap.Wallets[0].BalanceCents != 80000 {
		t.Fatalf("income should add to wallet balance, got %d", snap.Wallets[0].BalanceCents)
	}
}

func TestLedgerDeleteConfirmsAndRestoresBalance(t *testing.T) {
	a, store := newUITestApp(t)
	ctx := context.Background()
	wallet := a.snap.Wallets[0]
	entry, err := store.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 2500, OccurredAt: testNow})
	if err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt := testerFor(a)
	if err := tt.Click("流水"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("本月合计") || !tt.HasText("餐饮") || !tt.HasText("默认钱包") {
		t.Fatalf("ledger contents missing; texts: %v", tt.Texts())
	}
	if err := tt.Click(deleteEntryLabel(entry)); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("余额会加回") {
		t.Fatalf("delete confirmation does not explain the reversal; texts: %v", tt.Texts())
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	snap, err := store.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 0 || snap.Wallets[0].BalanceCents != 0 {
		t.Fatalf("after delete: ledger=%+v wallets=%+v", snap.Ledger, snap.Wallets)
	}
}

func TestFutureMonthIsEmptyNotAnError(t *testing.T) {
	a, _ := newUITestApp(t)
	tt := testerFor(a)
	if err := tt.Click("流水"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("下一月"); err != nil {
		t.Fatal(err)
	}
	if !a.ledgerMonth.Equal(AddMonths(testNow, 1)) {
		t.Fatalf("ledger month = %v, want next month", a.ledgerMonth)
	}
	if !tt.HasText("2026年12月合计") || !tt.HasText("这个月还没有流水") {
		t.Fatalf("future month empty state missing; texts: %v", tt.Texts())
	}
	if err := tt.Click("上一月"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("本月合计") {
		t.Fatalf("back to the current month: %v", tt.Texts())
	}
}

func TestAddWalletModal(t *testing.T) {
	a, store := newUITestApp(t)
	tt := testerFor(a)
	if err := tt.Click("钱包"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("添加钱包"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("钱包名字"); err != nil {
		t.Fatal(err)
	}
	tt.Type("现金")
	if err := tt.Click("初始余额"); err != nil {
		t.Fatal(err)
	}
	tt.Type("100")
	if err := tt.Click("确认添加钱包"); err != nil {
		t.Fatal(err)
	}
	snap, err := store.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	var cash *Wallet
	for i := range snap.Wallets {
		if snap.Wallets[i].Name == "现金" {
			cash = &snap.Wallets[i]
		}
	}
	if cash == nil || cash.BalanceCents != 10000 {
		t.Fatalf("wallets after add = %+v", snap.Wallets)
	}
	if a.form.walletID != cash.ID {
		t.Fatalf("new wallet was not selected: form=%d wallet=%d", a.form.walletID, cash.ID)
	}
}

func TestCalibrationModalRecordsDifference(t *testing.T) {
	a, store := newUITestApp(t)
	ctx := context.Background()
	wallet, err := store.CreateWallet(ctx, "零钱包", 10000)
	if err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt := testerFor(a)
	if err := tt.Click("钱包"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("校准余额：零钱包"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("实际余额"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("40")
	if err := tt.Click("确认校准余额"); err != nil {
		t.Fatal(err)
	}
	snap, err := store.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	var got *Wallet
	for i := range snap.Wallets {
		if snap.Wallets[i].ID == wallet.ID {
			got = &snap.Wallets[i]
		}
	}
	if got == nil || got.BalanceCents != 4000 {
		t.Fatalf("calibrated wallet = %+v", got)
	}
	if len(snap.Ledger) != 1 || !snap.Ledger[0].Adjustment || snap.Ledger[0].Category != adjustmentCategory {
		t.Fatalf("calibration entry = %+v", snap.Ledger)
	}
}

func TestCalibrationDeleteWarnsAboutNoRollback(t *testing.T) {
	a, store := newUITestApp(t)
	ctx := context.Background()
	wallet := a.snap.Wallets[0]
	if _, err := store.CalibrateWallet(ctx, wallet.ID, 5000); err != nil {
		t.Fatal(err)
	}
	a.reload()
	entry := a.snap.Ledger[0]
	tt := testerFor(a)
	if err := tt.Click("流水"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("校准") {
		t.Fatalf("adjustment badge missing; texts: %v", tt.Texts())
	}
	if err := tt.Click(deleteEntryLabel(entry)); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("删除后余额不会自动回滚") {
		t.Fatalf("calibration warning missing; texts: %v", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if len(a.snap.Ledger) != 1 {
		t.Fatal("canceling the alert deleted the entry")
	}
}

func TestAmountIsFocusedOnOpen(t *testing.T) {
	a, _ := newUITestApp(t)
	tt := testerFor(a)
	if !tt.Focused("金额") {
		t.Fatalf("amount input should be focused when the home page opens; focused texts: %v", tt.Texts())
	}
}

func TestWalletSelectorClickSetsDefaultAndOverdrawnSortsFirst(t *testing.T) {
	a, store := newUITestApp(t)
	ctx := context.Background()
	wallet, err := store.CreateWallet(ctx, "现金", 10000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateWallet(ctx, "储蓄卡", 50000); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 12000}); err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt := testerFor(a)
	if wallets := a.displayWallets(); len(wallets) == 0 || wallets[0].Name != "现金" {
		t.Fatalf("overdrawn wallet should sort first: %+v", wallets)
	}
	if !tt.HasText("透支") {
		t.Fatalf("overdrawn label missing; texts: %v", tt.Texts())
	}
	if err := tt.Click("选择钱包：现金"); err != nil {
		t.Fatal(err)
	}
	if a.form.walletID != wallet.ID {
		t.Fatalf("clicking a wallet status did not set entry default: %d", a.form.walletID)
	}
}

func TestStorageFailureKeepsEntryAndShowsNonBlockingMessage(t *testing.T) {
	a, store := newUITestApp(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	tt := testerFor(a)
	if err := tt.Click("金额"); err != nil {
		t.Fatal(err)
	}
	tt.Type("9.90")
	if err := tt.Click("分类：餐饮"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("投喂钱包"); err != nil {
		t.Fatal(err)
	}
	if a.form.amount != "9.90" || a.form.category != "餐饮" {
		t.Fatalf("failed save should keep the entry for a retry: %+v", a.form)
	}
	if a.hint == "" {
		t.Fatal("failed save should show a non-blocking hint")
	}
	if !tt.Focused("金额") {
		t.Fatal("focus should return to the amount after a failed save")
	}
}

func TestDeleteWalletWarnsCountAndKeepsHistory(t *testing.T) {
	a, store := newUITestApp(t)
	ctx := context.Background()
	wallet, err := store.CreateWallet(ctx, "现金", 10000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 2500}); err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt := testerFor(a)
	if err := tt.Click("钱包"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("删除钱包：现金"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("有 1 笔记录关联此钱包，删除后余额将不计入") {
		t.Fatalf("wallet deletion warning missing; texts: %v", tt.Texts())
	}
	// The page behind the alert also has a "删除" action; use the alert's
	// accessible button bounds instead of the first label on the page.
	del, ok := tt.AccessByRole(ui.RoleButton, "删除")
	if !ok {
		t.Fatalf("confirmation delete button missing; texts: %v", tt.Texts())
	}
	tt.ClickAt(del.Bounds.X+del.Bounds.W/2, del.Bounds.Y+del.Bounds.H/2)
	snap, err := store.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range snap.Wallets {
		if w.ID == wallet.ID {
			t.Fatalf("wallet still exists: %+v", snap.Wallets)
		}
	}
	if len(snap.Ledger) != 1 || snap.Ledger[0].WalletID != 0 || snap.Ledger[0].WalletName != "现金" {
		t.Fatalf("orphaned history = %+v", snap.Ledger)
	}
	if snap.WalletTotalCents != 0 {
		t.Fatalf("deleted wallet still counted in total: %d", snap.WalletTotalCents)
	}
}

func TestBudgetModalChangesMoodScale(t *testing.T) {
	a, store := newUITestApp(t)
	ctx := context.Background()
	if err := store.SetBudget(ctx, 500_000); err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt := testerFor(a)
	if !tt.HasText("月预算 ¥5,000") {
		t.Fatalf("budget chip missing; texts: %v", tt.Texts())
	}
	if err := tt.Click("调整月预算"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("月预算金额"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("2000")
	if err := tt.Click("确认更新预算"); err != nil {
		t.Fatal(err)
	}
	snap, err := store.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.BudgetCents != 200000 {
		t.Fatalf("budget = %d, want 200000", snap.BudgetCents)
	}
}

func TestNoteFieldKeepsFocus(t *testing.T) {
	a, _ := newUITestApp(t)
	tt := testerFor(a)
	if err := tt.Click("金额"); err != nil {
		t.Fatal(err)
	}
	tt.Type("12")
	if err := tt.Click("备注"); err != nil {
		t.Fatal(err)
	}
	tt.Type("午饭")
	if a.form.note != "午饭" {
		t.Fatalf("note = %q; the amount field stole focus", a.form.note)
	}
	if !tt.Focused("备注") {
		t.Fatalf("note field lost focus; texts: %v", tt.Texts())
	}
}
