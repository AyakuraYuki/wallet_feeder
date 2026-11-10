package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local)

func openTestStore(t testing.TB) *SQLiteStore {
	t.Helper()
	s, err := OpenSQLiteStore(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	s.now = func() time.Time { return testNow }
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStoreSeedsDefaultWalletAndBudget(t *testing.T) {
	s := openTestStore(t)
	snap, err := s.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Wallets) != 1 || snap.Wallets[0].Name != "默认钱包" || snap.Wallets[0].BalanceCents != 0 {
		t.Fatalf("wallets = %+v", snap.Wallets)
	}
	if snap.WalletTotalCents != 0 || snap.BudgetCents != defaultBudgetCents {
		t.Fatalf("total=%d budget=%d", snap.WalletTotalCents, snap.BudgetCents)
	}
}

func TestStoreSeedsDefaultWalletOnlyOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "walletfeeder.db")
	ctx := context.Background()

	s, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	wallet := s.walletsForTest(t, ctx)[0]
	if err := s.DeleteWallet(ctx, wallet.ID); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	s2, err := OpenSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	snap, err := s2.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Wallets) != 0 {
		t.Fatalf("wallets after reopen = %+v, want none", snap.Wallets)
	}
}

func TestAddExpenseAndIncomeUpdateBalanceAndTotals(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]

	if _, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 3650, Note: "午饭"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindIncome, Category: "工资", AmountCents: 200_00}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentStats.ExpenseCents != 3650 || snap.CurrentStats.IncomeCents != 20000 {
		t.Fatalf("stats = %+v", snap.CurrentStats)
	}
	if snap.Wallets[0].BalanceCents != 20000-3650 {
		t.Fatalf("balance = %d", snap.Wallets[0].BalanceCents)
	}
	if len(snap.Ledger) != 2 {
		t.Fatalf("ledger = %+v", snap.Ledger)
	}
	if snap.Ledger[0].WalletName != "默认钱包" || snap.Ledger[0].Note != "" {
		t.Fatalf("entry = %+v", snap.Ledger[0])
	}
}

func TestAddEntryRejectsInvalidWalletWithoutWriting(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.AddEntry(ctx, NewEntry{WalletID: 999, Kind: KindExpense, Category: "餐饮", AmountCents: 100}); !errors.Is(err, ErrWalletNotFound) {
		t.Fatalf("error = %v, want ErrWalletNotFound", err)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 0 || snap.CurrentStats.ExpenseCents != 0 {
		t.Fatalf("invalid write left data: %+v", snap)
	}
}

func TestDeleteExpenseRestoresBalance(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	if _, err := s.CreateWallet(ctx, "现金", 100_00); err != nil {
		t.Fatal(err)
	}
	wallet = s.walletsForTest(t, ctx)[1]
	entry, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "购物", AmountCents: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 0 {
		t.Fatalf("ledger = %+v, want empty", snap.Ledger)
	}
	if snap.Wallets[1].BalanceCents != 10000 {
		t.Fatalf("balance = %d, want 10000", snap.Wallets[1].BalanceCents)
	}
}

func TestDeleteIncomeSubtractsBalance(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	entry, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindIncome, Category: "红包", AmountCents: 800})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Wallets[0].BalanceCents != 0 {
		t.Fatalf("balance = %d, want 0", snap.Wallets[0].BalanceCents)
	}
}

func TestCalibrationCreatesAdjustmentAndDeleteDoesNotRollBack(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	if err := s.RenameWallet(ctx, wallet.ID, "工资卡"); err != nil {
		t.Fatal(err)
	}

	diff, err := s.CalibrateWallet(ctx, wallet.ID, 25_000)
	if err != nil {
		t.Fatal(err)
	}
	if diff != 25000 {
		t.Fatalf("diff = %d, want 25000", diff)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Wallets[0].BalanceCents != 25000 {
		t.Fatalf("balance = %d", snap.Wallets[0].BalanceCents)
	}
	if len(snap.Ledger) != 1 || !snap.Ledger[0].Adjustment || snap.Ledger[0].Category != adjustmentCategory {
		t.Fatalf("calibration entry = %+v", snap.Ledger)
	}
	if snap.CurrentStats.IncomeCents != 25000 {
		t.Fatalf("calibration income = %d", snap.CurrentStats.IncomeCents)
	}
	entry := snap.Ledger[0]

	if err := s.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}
	snap, err = s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Wallets[0].BalanceCents != 25000 {
		t.Fatalf("balance after deleting calibration = %d, want 25000", snap.Wallets[0].BalanceCents)
	}
	if len(snap.Ledger) != 0 {
		t.Fatalf("ledger = %+v, want calibration gone", snap.Ledger)
	}
}

func TestDeleteWalletKeepsOrphanedEntries(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet, err := s.CreateWallet(ctx, "零钱", 100_00)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "交通", AmountCents: 300})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWallet(ctx, wallet.ID); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.WalletTotalCents != 0 {
		t.Fatalf("wallet total = %d, want 0 (only default wallet remains)", snap.WalletTotalCents)
	}
	if len(snap.Ledger) != 1 {
		t.Fatalf("ledger = %+v, want the orphaned entry", snap.Ledger)
	}
	if snap.Ledger[0].ID != entry.ID || snap.Ledger[0].WalletID != 0 || snap.Ledger[0].WalletName != "零钱" {
		t.Fatalf("orphaned entry = %+v", snap.Ledger[0])
	}
	if err := s.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatalf("deleting orphaned entry: %v", err)
	}
}

func TestMonthlyFilterAndOrdering(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	oct := time.Date(2026, 10, 31, 23, 59, 0, 0, time.Local)
	novStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	novEnd := time.Date(2026, 11, 30, 23, 59, 0, 0, time.Local)
	dec := time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local)
	for _, at := range []time.Time{oct, novStart, novEnd, dec} {
		if _, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 100, OccurredAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	nov := time.Date(2026, 11, 15, 0, 0, 0, 0, time.Local)
	snap, err := s.Snapshot(ctx, nov, nov)
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentStats.ExpenseCents != 200 || len(snap.Ledger) != 2 {
		t.Fatalf("November stats=%+v ledger=%d, want 2 entries", snap.CurrentStats, len(snap.Ledger))
	}
	if !snap.Ledger[0].OccurredAt.Equal(novEnd) || !snap.Ledger[1].OccurredAt.Equal(novStart) {
		t.Fatalf("order = %v, %v", snap.Ledger[0].OccurredAt, snap.Ledger[1].OccurredAt)
	}
}

func TestCategories(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	for _, e := range []NewEntry{
		{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 500},
		{WalletID: wallet.ID, Kind: KindExpense, Category: "交通", AmountCents: 300},
		{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 200},
		{WalletID: wallet.ID, Kind: KindIncome, Category: "工资", AmountCents: 9999},
	} {
		if _, err := s.AddEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Categories) != 2 || snap.Categories[0].Category != "餐饮" || snap.Categories[0].AmountCents != 700 {
		t.Fatalf("categories = %+v", snap.Categories)
	}
}

func TestWalletsAndBudgetCRUD(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet, err := s.CreateWallet(ctx, "支付宝", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameWallet(ctx, wallet.ID, "手机"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameWallet(ctx, wallet.ID, ""); !errors.Is(err, ErrWalletName) {
		t.Fatalf("empty rename error = %v", err)
	}
	if err := s.SetBudget(ctx, 450_00); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.BudgetCents != 45000 {
		t.Fatalf("budget = %d", snap.BudgetCents)
	}
	if len(snap.Wallets) != 2 || snap.Wallets[1].Name != "手机" {
		t.Fatalf("wallets = %+v", snap.Wallets)
	}
	if err := s.SetBudget(ctx, 0); !errors.Is(err, ErrBudget) {
		t.Fatalf("zero budget error = %v", err)
	}
}

func TestWalletEntryCounts(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	for range 3 {
		if _, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 100}); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if snap.WalletEntryCount[wallet.ID] != 3 {
		t.Fatalf("count = %d", snap.WalletEntryCount[wallet.ID])
	}
}

func (s *SQLiteStore) walletsForTest(t *testing.T, ctx context.Context) []Wallet {
	t.Helper()
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return snap.Wallets
}

func TestRenameShowsNewNameInLedger(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	if _, err := s.AddEntry(ctx, NewEntry{WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 100}); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameWallet(ctx, wallet.ID, "饭卡"); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 1 || snap.Ledger[0].WalletName != "饭卡" {
		t.Fatalf("ledger wallet name = %+v, want 饭卡", snap.Ledger)
	}
}

func TestAddEntryRollsBackWhenBalanceUpdateFails(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	wallet := s.walletsForTest(t, ctx)[0]
	if _, err := s.db.Execute(ctx, `CREATE TRIGGER reject_balance_update
		BEFORE UPDATE ON wallets BEGIN SELECT RAISE(ABORT, 'forced failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddEntry(ctx, NewEntry{
		WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 1000,
	}); err == nil {
		t.Fatal("AddEntry succeeded despite the failing balance update")
	}
	snap, err := s.Snapshot(ctx, testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 0 {
		t.Fatalf("partial entry left behind: %+v", snap.Ledger)
	}
	if snap.Wallets[0].BalanceCents != 0 {
		t.Fatalf("partial balance update left behind: %d", snap.Wallets[0].BalanceCents)
	}
}
