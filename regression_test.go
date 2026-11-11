package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type snapshotFailureStore struct {
	Store
	fail bool
	err  error
}

func (s *snapshotFailureStore) Snapshot(ctx context.Context, currentMonth, ledgerMonth time.Time) (Snapshot, error) {
	if s.fail {
		return Snapshot{}, s.err
	}
	return s.Store.Snapshot(ctx, currentMonth, ledgerMonth)
}

func TestCommittedWriteRefreshFailureDoesNotRepeatMutation(t *testing.T) {
	base := openTestStore(t)
	store := &snapshotFailureStore{Store: base, err: errors.New("simulated snapshot failure")}
	a := NewApp(store)
	a.syncWork = true
	a.now = func() time.Time { return testNow }
	a.AttachStore(store)
	t.Cleanup(a.close)

	a.form.amount = "12.34"
	a.form.category = "餐饮"
	store.fail = true
	a.submitEntry(nil)

	snap, err := base.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ledger) != 1 || snap.CurrentStats.ExpenseCents != 1234 || snap.Ledger[0].WalletID == 0 {
		t.Fatalf("committed entry = %+v", snap.Ledger)
	}
	if a.form.amount != "" || a.form.category != "" {
		t.Fatalf("committed form was not cleared: %+v", a.form)
	}
	if a.hint == "没存上，请再试一次" || a.busy || !a.refreshNeeded {
		t.Fatalf("post-commit failure state: hint=%q busy=%v refreshNeeded=%v", a.hint, a.busy, a.refreshNeeded)
	}

	tt := testerFor(a)
	if !tt.HasText("刷新账本") {
		t.Fatalf("read-only recovery action missing: %v", tt.Texts())
	}
	store.fail = false
	if err := tt.Click("刷新账本"); err != nil {
		t.Fatal(err)
	}
	if a.refreshNeeded || len(a.snap.Ledger) != 1 || a.snap.CurrentStats.ExpenseCents != 1234 {
		t.Fatalf("refresh did not recover the committed write: %+v", a.snap)
	}

	// The same post-commit distinction applies to modal mutations. A newly
	// created wallet must not be created a second time on a refresh retry.
	a.openAddWallet()
	a.modalName = "现金"
	a.modalAmount = "10"
	store.fail = true
	a.submitModal()
	if a.modalOpen || a.modalError != "" {
		t.Fatalf("committed wallet presented as failed: open=%v error=%q", a.modalOpen, a.modalError)
	}
	snap, err = base.Snapshot(context.Background(), testNow, testNow)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, w := range snap.Wallets {
		if w.Name == "现金" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("cash wallet count = %d, want one", count)
	}
}

func TestFailedLedgerMonthChangeRestoresConsistentSnapshot(t *testing.T) {
	base := openTestStore(t)
	wallet, err := base.CreateWallet(context.Background(), "日常", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base.AddEntry(context.Background(), NewEntry{
		WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 800, OccurredAt: testNow,
	}); err != nil {
		t.Fatal(err)
	}
	store := &snapshotFailureStore{Store: base, err: errors.New("simulated month read failure")}
	a := NewApp(store)
	a.syncWork = true
	a.now = func() time.Time { return testNow }
	a.AttachStore(store)
	t.Cleanup(a.close)
	originalMonth := a.ledgerMonth

	store.fail = true
	a.shiftLedgerMonth(1)
	if !a.ledgerMonth.Equal(originalMonth) || !a.snap.LedgerMonth.Equal(originalMonth) {
		t.Fatalf("month mismatch: selected=%v snapshot=%v", a.ledgerMonth, a.snap.LedgerMonth)
	}
	if len(a.snap.Ledger) != 1 || a.snap.LedgerStats.ExpenseCents != 800 || a.snap.Ledger[0].OccurredAt.Before(monthStart(testNow)) {
		t.Fatalf("old snapshot was damaged: %+v", a.snap.Ledger)
	}
	a.page = pageLedger
	tt := testerFor(a)
	if !tt.HasText("本月合计") || !tt.HasText("刷新账本") {
		t.Fatalf("failed month change did not retain a consistent view: %v", tt.Texts())
	}
}

func TestMonthRolloverReloadsCurrentStats(t *testing.T) {
	a, base := newUITestApp(t)
	wallet := a.snap.Wallets[0]
	if _, err := base.AddEntry(context.Background(), NewEntry{
		WalletID: wallet.ID, Kind: KindExpense, Category: "餐饮", AmountCents: 800, OccurredAt: testNow,
	}); err != nil {
		t.Fatal(err)
	}
	a.reload()
	nextMonth := AddMonths(testNow, 1)
	if _, err := base.AddEntry(context.Background(), NewEntry{
		WalletID: wallet.ID, Kind: KindIncome, Category: "工资", AmountCents: 5000, OccurredAt: nextMonth,
	}); err != nil {
		t.Fatal(err)
	}
	tt := testerFor(a)
	if a.snap.CurrentStats.ExpenseCents != 800 || a.snap.CurrentStats.IncomeCents != 0 {
		t.Fatalf("unexpected starting stats: %+v", a.snap.CurrentStats)
	}

	a.now = func() time.Time { return nextMonth.Add(12 * 24 * time.Hour) }
	tt.Frame()
	if !a.snap.CurrentMonth.Equal(nextMonth) || a.snap.CurrentStats.ExpenseCents != 0 || a.snap.CurrentStats.IncomeCents != 5000 {
		t.Fatalf("month rollover was not refreshed: month=%v stats=%+v", a.snap.CurrentMonth, a.snap.CurrentStats)
	}
	if !a.snap.LedgerMonth.Equal(monthStart(testNow)) || a.snap.LedgerStats.ExpenseCents != 800 {
		t.Fatalf("ledger browsing month was unexpectedly changed: %+v", a.snap)
	}
}

type budgetFailureStore struct{ Store }

func (s *budgetFailureStore) SetBudget(context.Context, int64) error {
	return errors.New("simulated budget write failure")
}

func TestBusyModalCannotDismissAndKeepsError(t *testing.T) {
	a := NewApp(&budgetFailureStore{Store: openTestStore(t)})
	a.syncWork = true
	a.now = func() time.Time { return testNow }
	a.AttachStore(a.store)
	posted := make(chan func(), 1)
	a.dispatch = func(fn func()) { posted <- fn }
	a.syncWork = false
	a.openBudget()
	a.modalAmount = "2500"
	tt := testerFor(a)
	if err := tt.Click("确认更新预算"); err != nil {
		t.Fatal(err)
	}
	if !a.busy || !a.modalOpen {
		t.Fatalf("mutation did not start in modal: busy=%v open=%v", a.busy, a.modalOpen)
	}

	tt.Key(0, ui.KeyEscape)
	tt.ClickAt(2, 2)
	if !a.modalOpen {
		t.Fatal("Escape or backdrop dismissed a saving modal")
	}
	select {
	case finish := <-posted:
		finish()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for failed budget task")
	}
	tt.Frame()
	if a.busy || !a.modalOpen || a.modalAmount != "2500" {
		t.Fatalf("modal lost state after failure: busy=%v open=%v amount=%q", a.busy, a.modalOpen, a.modalAmount)
	}
	if !tt.HasText("simulated budget write failure") {
		t.Fatalf("write error is not visible in the retained modal: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if a.modalOpen {
		t.Fatal("idle modal did not allow dismissal")
	}
	a.close()
}

type blockingAddStore struct {
	Store
	entered chan struct{}
	release chan struct{}
}

func (s *blockingAddStore) AddEntry(context.Context, NewEntry) (Entry, error) {
	close(s.entered)
	<-s.release
	// Simulate a successful commit immediately before shutdown.
	return Entry{ID: 123}, nil
}

func TestUnchangedCalibrationDoesNotClaimEntryCreated(t *testing.T) {
	a, _ := newUITestApp(t)
	a.openCalibrateWallet(a.snap.Wallets[0])
	a.submitModal()
	if a.toast != "余额没有变化" {
		t.Fatalf("unchanged calibration toast = %q", a.toast)
	}
	if len(a.snap.Ledger) != 0 {
		t.Fatalf("unchanged calibration created an entry: %+v", a.snap.Ledger)
	}
}

func TestCloseWaitsForBackgroundMutationWithoutNilStore(t *testing.T) {
	base := openTestStore(t)
	store := &blockingAddStore{Store: base, entered: make(chan struct{}), release: make(chan struct{})}
	a := NewApp(store)
	posted := make(chan func(), 1)
	a.dispatch = func(fn func()) { posted <- fn }
	a.mutate(func(ctx context.Context, store Store) error {
		_, err := store.AddEntry(ctx, NewEntry{WalletID: 1, Kind: KindExpense, Category: "餐饮", AmountCents: 100})
		return err
	}, nil, nil)
	<-store.entered

	closed := make(chan struct{})
	go func() {
		a.close()
		close(closed)
	}()
	<-a.workCtx.Done()
	select {
	case <-closed:
		t.Fatal("close returned before the background mutation finished")
	default:
	}
	close(store.release)
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("close did not finish after releasing the task")
	}
	select {
	case finish := <-posted:
		finish() // A late UI callback must be harmless after shutdown.
	default:
		t.Fatal("mutation completion was not posted")
	}
	if a.store != nil || !a.closed {
		t.Fatalf("shutdown state: store=%v closed=%v", a.store, a.closed)
	}
	a.close() // Repeated OnQuit callbacks must be harmless.
}

func TestCloseCancelsStartupAndClosesOpenedStore(t *testing.T) {
	base := openTestStore(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	a := NewApp(nil)
	posted := make(chan func(), 1)
	a.dispatch = func(fn func()) { posted <- fn }
	a.openStore = func(context.Context, string) (Store, error) {
		close(entered)
		<-release
		return base, nil
	}
	a.start("ignored")
	<-entered
	closed := make(chan struct{})
	go func() {
		a.close()
		close(closed)
	}()
	<-a.workCtx.Done()
	close(release)
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("close did not join the startup task")
	}
	if a.store != nil || a.pendingStore != nil {
		t.Fatal("startup leaked a store after shutdown")
	}
	select {
	case finish := <-posted:
		finish()
	default:
		t.Fatal("startup completion was not posted")
	}
	if _, err := base.Snapshot(context.Background(), testNow, testNow); err == nil {
		t.Fatal("store returned by a canceled startup was not closed")
	}
}

func TestCloseOwnsStartupStoreBeforeUICallback(t *testing.T) {
	base := openTestStore(t)
	a := NewApp(nil)
	posted := make(chan func(), 1)
	a.dispatch = func(fn func()) { posted <- fn }
	a.openStore = func(context.Context, string) (Store, error) { return base, nil }
	a.start("ignored")
	var finish func()
	select {
	case finish = <-posted:
	case <-time.After(2 * time.Second):
		t.Fatal("startup did not post its result")
	}
	// Do not run the UI callback: close must take ownership of the pending
	// handle itself when the window has already gone away.
	a.close()
	finish()
	if a.store != nil || a.pendingStore != nil {
		t.Fatal("pending store survived shutdown")
	}
	if _, err := base.Snapshot(context.Background(), testNow, testNow); err == nil {
		t.Fatal("pending startup store was not closed")
	}
}
