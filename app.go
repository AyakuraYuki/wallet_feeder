package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

type appPage int

const (
	pageHome appPage = iota
	pageLedger
	pageWallets
)

type entryForm struct {
	kind     Kind
	amount   string
	category string
	note     string
	walletID int64
}

const (
	reactionDuration = 300 * time.Millisecond
	shakeDuration    = 300 * time.Millisecond
)

// feedReaction is the non-modal wink the wallet gives after a record is
// saved. It only lives in UI state; no animation engine dependency.
type feedReaction struct {
	kind        Kind
	walletName  string
	amountCents int64
	until       time.Time
}

// Modal kinds. A single modal open flag drives ui.Modal.
const (
	modalNone      = ""
	modalBudget    = "budget"
	modalAddWallet = "add-wallet"
	modalRename    = "rename-wallet"
	modalCalibrate = "calibrate-wallet"
)

// App is the window's complete state. It is only mutated on the main
// thread; background tasks hand their result back through dispatch.
type App struct {
	store      Store
	dispatch   func(func())
	syncWork   bool
	now        func() time.Time
	dbPath     string
	openStore  func(context.Context, string) (Store, error)
	workCtx    context.Context
	cancelWork context.CancelFunc
	workWG     sync.WaitGroup
	closed     bool

	page     appPage
	useGlass bool
	backdrop *auroraBackdrop

	ready         bool
	fatalErr      string
	busy          bool
	refreshing    bool
	refreshNeeded bool

	snap      Snapshot
	breakdown []CategoryTotal

	form               entryForm
	hint               string
	shakeCategoryUntil time.Time
	shakeWalletUntil   time.Time
	reaction           *feedReaction

	ledgerMonth        time.Time
	ledgerLoading      bool
	ledgerList         ui.ListState
	walletList         ui.ListState
	monthRefreshTarget time.Time

	amountRef   ui.Handle
	focusAmount bool

	modalOpen   bool
	modalKind   string
	modalTarget int64
	modalName   string
	modalAmount string
	modalError  string

	deleteEntryOpen  bool
	deleteEntryID    int64
	deleteWalletOpen bool
	deleteWalletID   int64

	toast        string
	seq          uint64
	dataMu       sync.Mutex
	pendingStore Store // Owned by dataMu until the startup callback takes it.
}

// NewApp prepares the UI state around a persistence boundary. Production
// calls Start; tests inject an in-memory store and enable syncWork.
func NewApp(store Store) *App {
	now := time.Now
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		store:       store,
		now:         now,
		form:        entryForm{kind: KindExpense},
		ledgerMonth: monthStart(now()),
		page:        pageHome,
		useGlass:    true,
		openStore: func(ctx context.Context, path string) (Store, error) {
			return OpenSQLiteStore(ctx, path)
		},
		workCtx:    ctx,
		cancelWork: cancel,
	}
	if store != nil {
		a.ready = true
	}
	return a
}

func (a *App) currentMonth() time.Time { return monthStart(a.now()) }

// post hands work back to the main thread. In tests and in a headless view
// test there is no window, so it runs inline.
func (a *App) post(fn func()) {
	if a.syncWork || a.dispatch == nil {
		fn()
		return
	}
	a.dispatch(fn)
}

// start opens the SQLite file off the main thread so the window can be
// interactive immediately. It is also the retry path after a startup failure.
func (a *App) start(path string) {
	if a.closed {
		return
	}
	a.dbPath = path
	a.fatalErr = ""
	a.ready = false
	ctx := a.workCtx
	a.workWG.Go(func() {
		started, err := a.openStore(ctx, path)
		// Register the new handle before handing its ownership to the UI.
		// close can therefore dispose of it even when the window is gone
		// before the queued startup callback runs.
		a.dataMu.Lock()
		active := err == nil && ctx.Err() == nil
		if active {
			a.pendingStore = started
		}
		a.dataMu.Unlock()
		if err == nil && !active {
			_ = started.Close()
		}
		a.post(func() {
			if a.closed || ctx.Err() != nil {
				return
			}
			if err != nil {
				a.fatalErr = friendlyError(err)
				return
			}
			a.dataMu.Lock()
			a.pendingStore = nil
			a.dataMu.Unlock()
			a.store = started
			a.reload()
		})
	})
}

// AttachStore is the synchronous testing seam for a store that is already
// open.
func (a *App) AttachStore(store Store) {
	if a.closed {
		return
	}
	a.store = store
	a.ready = true
	a.reload()
}

func (a *App) retry() {
	if a.closed {
		return
	}
	a.fatalErr = ""
	a.ready = false
	if a.store != nil {
		a.reload()
		return
	}
	if a.dbPath == "" {
		return
	}
	a.start(a.dbPath)
}

// close stops new work, cancels and joins every background operation, then
// closes the store. MyGo invokes this on the UI thread, so posted callbacks
// cannot race this state transition.
func (a *App) close() {
	if a.closed {
		return
	}
	a.closed = true
	a.cancelWork()
	a.workWG.Wait()
	a.dataMu.Lock()
	pending := a.pendingStore
	a.pendingStore = nil
	a.dataMu.Unlock()
	if pending != nil {
		_ = pending.Close()
	}
	if a.store != nil {
		_ = a.store.Close()
		a.store = nil
	}
	a.ready = false
}

type taskOptions struct {
	writing       bool
	restoreLedger bool
	onSuccess     func()
	onError       func(error)
}

// refreshError marks a committed write whose subsequent snapshot failed.
// It is deliberately distinct from a write failure: retrying the former
// must never repeat the mutation.
type refreshError struct{ err error }

func (e *refreshError) Error() string { return "refresh after commit: " + e.err.Error() }
func (e *refreshError) Unwrap() error { return e.err }

func (a *App) beginTask(opts taskOptions) uint64 {
	a.seq++
	if opts.writing {
		a.busy = true
	} else {
		a.refreshing = true
	}
	return a.seq
}

func (a *App) runTask(store Store, work func(context.Context, Store) (Snapshot, error), opts taskOptions) {
	if a.closed || store == nil {
		return
	}
	seq := a.beginTask(opts)
	run := func() {
		// Single-instance mode plus this mutex keeps writes from committing
		// between the reads that make up one window snapshot.
		a.dataMu.Lock()
		snap, err := work(a.workCtx, store)
		a.dataMu.Unlock()
		a.post(func() { a.finishTask(seq, snap, err, opts) })
	}
	if a.syncWork {
		run()
		return
	}
	a.workWG.Go(func() {
		run()
	})
}

func (a *App) finishTask(seq uint64, snap Snapshot, err error, opts taskOptions) {
	if a.closed || seq != a.seq {
		return
	}
	if opts.writing {
		a.busy = false
	} else {
		a.refreshing = false
	}
	a.ledgerLoading = false
	if err != nil {
		if opts.restoreLedger && a.ready && !a.snap.LedgerMonth.IsZero() {
			// Keep the title, totals and entries on the same, last good month.
			a.ledgerMonth = a.snap.LedgerMonth
		}
		if !a.ready && a.fatalErr == "" {
			a.fatalErr = friendlyError(err)
			return
		}
		var refreshErr *refreshError
		if opts.writing && errors.As(err, &refreshErr) {
			a.refreshNeeded = true
			// The mutation is durable. Apply success-side UI cleanup (such
			// as clearing the form), but never claim that its write failed.
			if opts.onSuccess != nil {
				opts.onSuccess()
			}
			a.toast = "已保存，但页面刷新失败，请点“刷新账本”"
			return
		}
		if !opts.writing {
			a.refreshNeeded = true
		}
		if opts.onError != nil {
			opts.onError(err)
		} else {
			a.errorToast(err)
		}
		return
	}
	a.applySnapshot(snap)
	if opts.onSuccess != nil {
		opts.onSuccess()
	}
}

// reload reads both the current month and the ledger month. Writes call it
// after their transaction so the whole window updates from one snapshot.
func (a *App) reload() {
	if a.closed || a.store == nil {
		return
	}
	store, current, ledger := a.store, a.currentMonth(), a.ledgerMonth
	a.runTask(store, func(ctx context.Context, store Store) (Snapshot, error) {
		return store.Snapshot(ctx, current, ledger)
	}, taskOptions{restoreLedger: true})
}

// mutate runs a transaction on a background goroutine and, in the same
// task, loads the snapshot that follows it. The onSuccess callback runs on
// the main thread even if only the refresh failed, because the write itself
// has already committed and must not be repeated.
func (a *App) mutate(work func(context.Context, Store) error, onSuccess func(), onError func(error)) {
	if a.closed || a.store == nil || a.busy {
		return
	}
	store := a.store
	current, ledger := a.currentMonth(), a.ledgerMonth
	a.runTask(store, func(ctx context.Context, store Store) (Snapshot, error) {
		if err := work(ctx, store); err != nil {
			return Snapshot{}, err
		}
		snap, err := store.Snapshot(ctx, current, ledger)
		if err != nil {
			return Snapshot{}, &refreshError{err: err}
		}
		return snap, nil
	}, taskOptions{writing: true, onSuccess: onSuccess, onError: onError})
}

func (a *App) applySnapshot(snap Snapshot) {
	a.snap = snap
	a.ready = true
	a.refreshNeeded = false
	a.ledgerLoading = false
	if !snap.LedgerMonth.IsZero() {
		a.ledgerMonth = snap.LedgerMonth
	}
	a.breakdown = categoryBreakdown(snap.Categories, snap.CurrentStats.ExpenseCents)
	a.ensureSelectedWallet()
}

// refreshAtMonthBoundary schedules one redraw at the next month, rather than
// polling. A changed month is requested once; a failed read leaves the last
// good snapshot visible and offers the explicit refresh action.
func (a *App) refreshAtMonthBoundary(c *ui.Context) {
	if !a.ready || a.closed {
		return
	}
	now := a.now()
	current := monthStart(now)
	if until := current.AddDate(0, 1, 0).Sub(now); until > 0 {
		c.After(until)
	}
	if a.busy || a.snap.CurrentMonth.IsZero() || a.snap.CurrentMonth.Equal(current) ||
		a.monthRefreshTarget.Equal(current) {
		return
	}
	a.monthRefreshTarget = current
	a.reload()
}

func (a *App) ensureSelectedWallet() {
	for _, w := range a.snap.Wallets {
		if w.ID == a.form.walletID {
			return
		}
	}
	if len(a.snap.Wallets) > 0 {
		a.form.walletID = a.snap.Wallets[0].ID
	} else {
		a.form.walletID = 0
	}
}

func (a *App) selectedWallet() (Wallet, bool) {
	for _, w := range a.snap.Wallets {
		if w.ID == a.form.walletID {
			return w, true
		}
	}
	return Wallet{}, false
}

// displayWallets puts overdrawn wallets first, then the emptiest, so the
// wallets that need attention are the first thing the user sees.
func (a *App) displayWallets() []Wallet {
	out := append([]Wallet(nil), a.snap.Wallets...)
	sort.SliceStable(out, func(i, j int) bool {
		ni, nj := out[i].BalanceCents < 0, out[j].BalanceCents < 0
		if ni != nj {
			return ni
		}
		if out[i].BalanceCents != out[j].BalanceCents {
			return out[i].BalanceCents < out[j].BalanceCents
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (a *App) setPage(p appPage) {
	if a.page == p {
		return
	}
	a.page = p
	if p == pageHome {
		a.focusAmount = true
	} else {
		a.reaction = nil
	}
}

// --- Quick entry ---------------------------------------------------------

func (a *App) amountCents() (int64, bool) {
	v, err := ParsePositiveCents(a.form.amount)
	return v, err == nil
}

func (a *App) amountInvalidMessage() string {
	raw := strings.TrimSpace(a.form.amount)
	if raw == "" {
		return ""
	}
	if _, err := ParsePositiveCents(raw); err != nil {
		return "金额要大于 0，最多两位小数"
	}
	return ""
}

func (a *App) setKind(kind Kind) {
	if a.form.kind == kind {
		return
	}
	a.form.kind = kind
	a.form.category = ""
	a.hint = ""
	a.focusAmount = true
}

func (a *App) submitEntry(c *ui.Context) {
	if a.busy || a.store == nil {
		return
	}
	cents, err := ParsePositiveCents(a.form.amount)
	if err != nil {
		a.hint = "金额要大于 0，最多两位小数"
		return
	}
	wallet, ok := a.selectedWallet()
	if !ok {
		a.hint = "还没有钱包，先去钱包页添加一只"
		a.shakeWalletUntil = c.Now().Add(shakeDuration)
		return
	}
	if a.form.category == "" {
		a.hint = "先挑一个分类，钱包就开饭了"
		a.shakeCategoryUntil = c.Now().Add(shakeDuration)
		return
	}

	validCategory := false
	for _, cat := range categoriesFor(a.form.kind) {
		if cat.Name == a.form.category {
			validCategory = true
			break
		}
	}
	if !validCategory {
		a.hint = "分类和记账类型不匹配，请重新选择"
		return
	}

	in := NewEntry{
		WalletID:    wallet.ID,
		Kind:        a.form.kind,
		Category:    a.form.category,
		AmountCents: cents,
		Note:        strings.TrimSpace(a.form.note),
		OccurredAt:  a.now(),
	}
	a.hint = ""
	a.mutate(func(ctx context.Context, store Store) error {
		_, err := store.AddEntry(ctx, in)
		return err
	}, func() {
		a.form.amount = ""
		a.form.category = ""
		a.form.note = ""
		a.hint = ""
		a.shakeCategoryUntil = time.Time{}
		a.shakeWalletUntil = time.Time{}
		a.reaction = &feedReaction{
			kind:        in.Kind,
			walletName:  wallet.Name,
			amountCents: in.AmountCents,
		}
		a.focusAmount = true
	}, func(err error) {
		a.hint = "没存上，请再试一次"
		a.focusAmount = true
		a.errorToast(err)
	})
}

// activeReaction clears the feeding reaction when its 300 ms are over and
// asks MyGo for a frame at that moment. The window never polls.
func (a *App) activeReaction(c *ui.Context) *feedReaction {
	if a.reaction == nil {
		return nil
	}
	now := c.Now()
	if a.reaction.until.IsZero() {
		a.reaction.until = now.Add(reactionDuration)
	}
	if !now.Before(a.reaction.until) {
		a.reaction = nil
		return nil
	}
	c.After(a.reaction.until.Sub(now))
	return a.reaction
}

// shakeOffset animates the little "no, not that one" wiggle of a missing
// selector. Time comes from the window's clock, which tests control.
func shakeOffset(c *ui.Context, until *time.Time) float32 {
	if until.IsZero() {
		return 0
	}
	now := c.Now()
	if !now.Before(*until) {
		*until = time.Time{}
		return 0
	}
	c.After(16 * time.Millisecond)
	left := float32(until.Sub(now).Milliseconds())
	return 5 * float32(math.Sin(float64(left)/24))
}

// --- Ledger --------------------------------------------------------------

func (a *App) shiftLedgerMonth(delta int) {
	if a.busy {
		return
	}
	a.ledgerMonth = AddMonths(a.ledgerMonth, delta)
	a.ledgerList = ui.ListState{}
	a.ledgerLoading = a.store != nil
	a.reload()
}

func (a *App) goToCurrentMonth() {
	if a.busy {
		return
	}
	target := a.currentMonth()
	if target.Equal(a.ledgerMonth) {
		return
	}
	a.ledgerMonth = target
	a.ledgerList = ui.ListState{}
	a.ledgerLoading = a.store != nil
	a.reload()
}

func (a *App) askDeleteEntry(id int64) {
	if a.busy {
		return
	}
	a.deleteEntryID = id
	a.deleteEntryOpen = true
}

func (a *App) confirmDeleteEntry(entry Entry) {
	a.mutate(func(ctx context.Context, store Store) error {
		return store.DeleteEntry(ctx, entry.ID)
	}, func() {
		a.toast = "已删除这笔记录"
	}, func(err error) {
		a.errorToast(err)
	})
}

func (a *App) askDeleteWallet(id int64) {
	if a.busy {
		return
	}
	a.deleteWalletID = id
	a.deleteWalletOpen = true
}

func (a *App) confirmDeleteWallet(id int64) {
	a.mutate(func(ctx context.Context, store Store) error {
		return store.DeleteWallet(ctx, id)
	}, func() {
		a.toast = "钱包已删除，相关流水仍保留"
	}, func(err error) {
		a.errorToast(err)
	})
}

// --- Wallets and budget --------------------------------------------------

func (a *App) openAddWallet() {
	if a.busy {
		return
	}
	a.modalOpen = true
	a.modalKind = modalAddWallet
	a.modalTarget = 0
	a.modalName = ""
	a.modalAmount = ""
	a.modalError = ""
}

func (a *App) openRenameWallet(w Wallet) {
	if a.busy {
		return
	}
	a.modalOpen = true
	a.modalKind = modalRename
	a.modalTarget = w.ID
	a.modalName = w.Name
	a.modalAmount = ""
	a.modalError = ""
}

func (a *App) openCalibrateWallet(w Wallet) {
	if a.busy {
		return
	}
	a.modalOpen = true
	a.modalKind = modalCalibrate
	a.modalTarget = w.ID
	a.modalName = w.Name
	a.modalAmount = formatPlainCents(w.BalanceCents)
	a.modalError = ""
}

func (a *App) openBudget() {
	if a.busy {
		return
	}
	a.modalOpen = true
	a.modalKind = modalBudget
	a.modalTarget = 0
	a.modalName = ""
	a.modalAmount = formatPlainCents(a.snap.BudgetCents)
	a.modalError = ""
}

func (a *App) closeModal() {
	a.modalOpen = false
	a.modalKind = modalNone
	a.modalError = ""
}

func (a *App) submitModal() {
	if a.busy || !a.modalOpen {
		return
	}
	switch a.modalKind {
	case modalBudget:
		cents, err := ParsePositiveCents(a.modalAmount)
		if err != nil {
			a.modalError = "预算要大于 0，最多两位小数"
			return
		}
		if cents == a.snap.BudgetCents {
			a.closeModal()
			a.toast = "月预算没有变化"
			return
		}
		a.mutate(func(ctx context.Context, store Store) error {
			return store.SetBudget(ctx, cents)
		}, func() {
			a.closeModal()
			a.toast = "月预算已更新"
		}, func(err error) {
			a.modalError = friendlyError(err)
		})
	case modalAddWallet:
		name := strings.TrimSpace(a.modalName)
		if name == "" {
			a.modalError = "先给钱包取个名字"
			return
		}
		initial := int64(0)
		var err error
		if strings.TrimSpace(a.modalAmount) != "" {
			initial, err = ParseBalanceCents(a.modalAmount)
			if err != nil || initial < 0 {
				a.modalError = "初始余额要大于等于 0，最多两位小数"
				return
			}
		}
		var createdID int64
		a.mutate(func(ctx context.Context, store Store) error {
			w, err := store.CreateWallet(ctx, name, initial)
			createdID = w.ID
			return err
		}, func() {
			if createdID > 0 {
				a.form.walletID = createdID
			}
			a.closeModal()
			a.toast = "新钱包已经住进来啦"
		}, func(err error) {
			a.modalError = friendlyError(err)
		})
	case modalRename:
		name := strings.TrimSpace(a.modalName)
		if name == "" {
			a.modalError = "名字不能是空的"
			return
		}
		target := a.modalTarget
		a.mutate(func(ctx context.Context, store Store) error {
			return store.RenameWallet(ctx, target, name)
		}, func() {
			a.closeModal()
			a.toast = "名字已更新"
		}, func(err error) {
			a.modalError = friendlyError(err)
		})
	case modalCalibrate:
		targetCents, err := ParseBalanceCents(a.modalAmount)
		if err != nil {
			a.modalError = "请输入实际余额，可为负数"
			return
		}
		target := a.modalTarget
		var difference int64
		a.mutate(func(ctx context.Context, store Store) error {
			var err error
			difference, err = store.CalibrateWallet(ctx, target, targetCents)
			return err
		}, func() {
			a.closeModal()
			if difference == 0 {
				a.toast = "余额没有变化"
			} else {
				a.toast = "余额已校准，调整差额已记入流水"
			}
		}, func(err error) {
			a.modalError = friendlyError(err)
		})
	}
}

func (a *App) walletByID(id int64) (Wallet, bool) {
	for _, w := range a.snap.Wallets {
		if w.ID == id {
			return w, true
		}
	}
	return Wallet{}, false
}

func (a *App) entryByID(id int64) (Entry, bool) {
	for _, e := range a.snap.Ledger {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

func (a *App) errorToast(err error) {
	a.toast = friendlyError(err)
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "操作超时，请再试一次"
	}
	switch {
	case errors.Is(err, ErrWalletName):
		return "钱包名字不能为空"
	case errors.Is(err, ErrWalletNotFound):
		return "这只钱包已经不在了"
	case errors.Is(err, ErrAmountTooPrecise):
		return "金额最多两位小数"
	case errors.Is(err, ErrAmountNotPositive):
		return "金额要大于 0"
	case errors.Is(err, ErrBudget):
		return "月预算要大于 0"
	}
	msg := err.Error()
	if len([]rune(msg)) > 80 {
		msg = string([]rune(msg)[:80]) + "…"
	}
	return "操作没有完成：" + msg
}

// formatPlainCents is the editable form of a balance, without a currency
// symbol, so a text field can select and replace it easily.
func formatPlainCents(cents int64) string {
	sign := ""
	whole, frac := cents/centsPerYuan, cents%centsPerYuan
	if cents < 0 {
		sign = "-"
		whole, frac = -whole, -frac
	}
	return fmt.Sprintf("%s%d.%02d", sign, whole, frac)
}
