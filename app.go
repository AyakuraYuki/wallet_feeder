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
	store    Store
	dispatch func(func())
	syncWork bool
	now      func() time.Time
	dbPath   string

	page     appPage
	useGlass bool

	ready    bool
	fatalErr string
	busy     bool

	snap      Snapshot
	breakdown []CategoryTotal

	form               entryForm
	hint               string
	shakeCategoryUntil time.Time
	shakeWalletUntil   time.Time
	reaction           *feedReaction

	ledgerMonth   time.Time
	ledgerLoading bool
	ledgerList    ui.ListState
	walletList    ui.ListState

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

	toast  string
	seq    uint64
	dataMu sync.Mutex
}

// NewApp prepares the UI state around a persistence boundary. Production
// calls Start; tests inject an in-memory store and enable syncWork.
func NewApp(store Store) *App {
	now := time.Now
	a := &App{
		store:       store,
		now:         now,
		form:        entryForm{kind: KindExpense},
		ledgerMonth: monthStart(now()),
		page:        pageHome,
		useGlass:    true,
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
// interactive immediately. It is also the retry path after a startup
// failure.
func (a *App) start(path string) {
	a.dbPath = path
	a.fatalErr = ""
	a.ready = false
	go func() {
		ctx := context.Background()
		store, err := OpenSQLiteStore(ctx, path)
		a.post(func() {
			if err != nil {
				a.fatalErr = friendlyError(err)
				return
			}
			a.store = store
			a.reload()
		})
	}()
}

// AttachStore is the synchronous testing seam for a store that is already
// open.
func (a *App) AttachStore(store Store) {
	a.store = store
	a.ready = true
	a.reload()
}

func (a *App) retry() {
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

func (a *App) close() {
	if a.store != nil {
		_ = a.store.Close()
		a.store = nil
	}
}

type taskOptions struct {
	writing   bool
	onSuccess func()
	onError   func(error)
}

func (a *App) beginTask(opts taskOptions) uint64 {
	a.seq++
	if opts.writing {
		a.busy = true
	}
	return a.seq
}

func (a *App) runTask(work func(context.Context) (Snapshot, error), opts taskOptions) {
	if a.store == nil && !opts.writing {
		return
	}
	seq := a.beginTask(opts)
	run := func() {
		// Snapshot is a batch of reads, not one SQLite transaction. The
		// mutex keeps a write from committing between those reads, so the
		// window never shows a half-updated month.
		a.dataMu.Lock()
		snap, err := work(context.Background())
		a.dataMu.Unlock()
		a.post(func() { a.finishTask(seq, snap, err, opts) })
	}
	if a.syncWork {
		run()
		return
	}
	go run()
}

func (a *App) finishTask(seq uint64, snap Snapshot, err error, opts taskOptions) {
	if seq != a.seq {
		return
	}
	if opts.writing {
		a.busy = false
	}
	a.ledgerLoading = false
	if err != nil {
		if !a.ready && a.fatalErr == "" {
			a.fatalErr = friendlyError(err)
			return
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
	if a.store == nil {
		return
	}
	current, ledger := a.currentMonth(), a.ledgerMonth
	a.runTask(func(ctx context.Context) (Snapshot, error) {
		return a.store.Snapshot(ctx, current, ledger)
	}, taskOptions{})
}

// mutate runs a transaction on a background goroutine and, in the same
// task, loads the snapshot that follows it. The onSuccess callback runs on
// the main thread after the new snapshot is installed.
func (a *App) mutate(work func(context.Context) error, onSuccess func(), onError func(error)) {
	if a.store == nil || a.busy {
		return
	}
	current, ledger := a.currentMonth(), a.ledgerMonth
	a.runTask(func(ctx context.Context) (Snapshot, error) {
		if err := work(ctx); err != nil {
			return Snapshot{}, err
		}
		return a.store.Snapshot(ctx, current, ledger)
	}, taskOptions{writing: true, onSuccess: onSuccess, onError: onError})
}

func (a *App) applySnapshot(snap Snapshot) {
	a.snap = snap
	a.ready = true
	a.ledgerLoading = false
	if !snap.LedgerMonth.IsZero() {
		a.ledgerMonth = snap.LedgerMonth
	}
	a.breakdown = categoryBreakdown(snap.Categories, snap.CurrentStats.ExpenseCents)
	a.ensureSelectedWallet()
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
	a.mutate(func(ctx context.Context) error {
		_, err := a.store.AddEntry(ctx, in)
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
	a.mutate(func(ctx context.Context) error {
		return a.store.DeleteEntry(ctx, entry.ID)
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
	a.mutate(func(ctx context.Context) error {
		return a.store.DeleteWallet(ctx, id)
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
		a.mutate(func(ctx context.Context) error {
			return a.store.SetBudget(ctx, cents)
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
		a.mutate(func(ctx context.Context) error {
			w, err := a.store.CreateWallet(ctx, name, initial)
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
		a.mutate(func(ctx context.Context) error {
			return a.store.RenameWallet(ctx, target, name)
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
		a.mutate(func(ctx context.Context) error {
			_, err := a.store.CalibrateWallet(ctx, target, targetCents)
			return err
		}, func() {
			a.closeModal()
			a.toast = "余额已校准，调整差额已记入流水"
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
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/centsPerYuan, cents%centsPerYuan)
}
