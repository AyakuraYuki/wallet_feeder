package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/sqlite"
)

const (
	defaultBudgetCents int64 = 300_000 // ¥3,000
	maxWalletNameRunes       = 24
	maxNoteRunes             = 80
)

var (
	ErrWalletName       = errors.New("wallet name must not be empty")
	ErrWalletNotFound   = errors.New("wallet not found")
	ErrWalletInitial    = errors.New("wallet initial balance must be zero or positive")
	ErrBudget           = errors.New("budget must be greater than zero")
	ErrEntryCategory    = errors.New("category must not be empty")
	ErrEntryNoteTooLong = errors.New("note is too long")
)

// Wallet is a place money lives.
type Wallet struct {
	ID           int64
	Name         string
	BalanceCents int64
	CreatedAt    time.Time
}

// Entry is one line of the ledger: a meal for the wallet, or money it
// received. Adjustment marks a balance calibration, which is never rolled
// back automatically.
type Entry struct {
	ID          int64
	WalletID    int64 // 0 once its wallet was deleted
	WalletName  string
	Kind        Kind
	Category    string
	AmountCents int64
	Note        string
	Adjustment  bool
	OccurredAt  time.Time
	CreatedAt   time.Time
}

// MonthStats are the two totals the month view needs.
type MonthStats struct {
	ExpenseCents int64
	IncomeCents  int64
}

// Snapshot is everything the window draws, read in one batch. Keeping it
// whole means a background refresh can never leave half of the screen on
// the old month.
type Snapshot struct {
	Wallets          []Wallet
	WalletTotalCents int64
	BudgetCents      int64
	WalletEntryCount map[int64]int

	CurrentMonth time.Time
	CurrentStats MonthStats
	Categories   []CategoryTotal

	LedgerMonth time.Time
	LedgerStats MonthStats
	Ledger      []Entry
}

// NewEntry is the input to AddEntry.
type NewEntry struct {
	WalletID    int64
	Kind        Kind
	Category    string
	AmountCents int64
	Note        string
	OccurredAt  time.Time
}

// Store is the persistence boundary. The window's actions call it on a
// background goroutine and apply its result on the main thread.
type Store interface {
	Snapshot(ctx context.Context, currentMonth, ledgerMonth time.Time) (Snapshot, error)
	AddEntry(ctx context.Context, in NewEntry) (Entry, error)
	DeleteEntry(ctx context.Context, id int64) error
	CreateWallet(ctx context.Context, name string, initialCents int64) (Wallet, error)
	RenameWallet(ctx context.Context, id int64, name string) error
	DeleteWallet(ctx context.Context, id int64) error
	CalibrateWallet(ctx context.Context, id, targetCents int64) (int64, error)
	SetBudget(ctx context.Context, budgetCents int64) error
	Close() error
}

// SQLiteStore stores WalletFeeder's data in the MyGo SQLite plugin's local
// file. Every operation that changes a balance uses one transaction.
type SQLiteStore struct {
	db  *sqlite.DB
	now func() time.Time
}

var _ Store = (*SQLiteStore)(nil)

// secureSQLiteFile creates the ledger with owner-only permissions and also
// tightens files made by older releases. Chmod the descriptor rather than
// the path so a symlink cannot chmod another file.
func secureSQLiteFile(path string) error {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("sqlite: database must be a regular file without symlinks: %s", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("sqlite: inspect database: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("sqlite: create private database: %w", err)
	}
	if info, statErr := f.Stat(); statErr != nil {
		_ = f.Close()
		return fmt.Errorf("sqlite: inspect open database: %w", statErr)
	} else if !info.Mode().IsRegular() {
		_ = f.Close()
		return fmt.Errorf("sqlite: database is not a regular file: %s", path)
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("sqlite: protect database: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("sqlite: close protected database: %w", err)
	}
	return nil
}

// OpenSQLiteStore opens path, applies the schema and seeds the first
// wallet. The parent directory must already exist.
func OpenSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	if path != ":memory:" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := secureSQLiteFile(path); err != nil {
			return nil, err
		}
	}
	db, err := sqlite.Open(ctx, path, sqlite.OpenOptions{BusyTimeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	s := &SQLiteStore{db: db, now: time.Now}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.bootstrap(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) migrate(ctx context.Context) error {
	_, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `CREATE TABLE IF NOT EXISTS wallets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			balance_cents INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`},
		{SQL: `CREATE TABLE IF NOT EXISTS entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			wallet_id INTEGER REFERENCES wallets(id) ON DELETE SET NULL,
			wallet_name TEXT NOT NULL,
			kind TEXT NOT NULL CHECK (kind IN ('expense', 'income')),
			category TEXT NOT NULL,
			amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
			note TEXT NOT NULL DEFAULT '',
			is_adjustment INTEGER NOT NULL DEFAULT 0 CHECK (is_adjustment IN (0, 1)),
			occurred_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		)`},
		{SQL: `CREATE INDEX IF NOT EXISTS entries_occurred ON entries(occurred_at)`},
		{SQL: `CREATE INDEX IF NOT EXISTS entries_wallet ON entries(wallet_id)`},
		{SQL: `CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`},
	})
	return err
}

func (s *SQLiteStore) bootstrap(ctx context.Context) error {
	now := s.now().UnixMilli()
	_, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `INSERT INTO wallets(name, balance_cents, created_at)
			SELECT '默认钱包', 0, ?1
			WHERE NOT EXISTS (SELECT 1 FROM wallets)
			  AND NOT EXISTS (SELECT 1 FROM settings WHERE key = 'bootstrapped')`,
			Args: []any{now}},
		{SQL: `INSERT OR IGNORE INTO settings(key, value) VALUES ('bootstrapped', '1')`},
		{SQL: `INSERT OR IGNORE INTO settings(key, value) VALUES ('monthly_budget_cents', ?1)`,
			Args: []any{strconv.FormatInt(defaultBudgetCents, 10)}},
	})
	return err
}

// Snapshot reads the current-month overview, all wallets and one ledger
// month at once. App serializes this batch against its own writes and main
// holds a single-instance lock; the connection itself has no read transaction.
func (s *SQLiteStore) Snapshot(ctx context.Context, currentMonth, ledgerMonth time.Time) (Snapshot, error) {
	var snap Snapshot
	snap.CurrentMonth = monthStart(currentMonth)
	snap.LedgerMonth = monthStart(ledgerMonth)

	wallets, err := s.wallets(ctx)
	if err != nil {
		return snap, err
	}
	snap.Wallets = wallets
	for _, w := range wallets {
		snap.WalletTotalCents += w.BalanceCents
	}

	if snap.BudgetCents, err = s.budget(ctx); err != nil {
		return snap, err
	}
	if snap.CurrentStats, err = s.monthStats(ctx, snap.CurrentMonth); err != nil {
		return snap, err
	}
	if snap.Categories, err = s.categories(ctx, snap.CurrentMonth); err != nil {
		return snap, err
	}
	if snap.LedgerStats, err = s.monthStats(ctx, snap.LedgerMonth); err != nil {
		return snap, err
	}
	if snap.Ledger, err = s.entries(ctx, snap.LedgerMonth); err != nil {
		return snap, err
	}
	if snap.WalletEntryCount, err = s.walletEntryCounts(ctx); err != nil {
		return snap, err
	}
	return snap, nil
}

func (s *SQLiteStore) wallets(ctx context.Context) ([]Wallet, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, balance_cents, created_at FROM wallets ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	out := make([]Wallet, 0, len(rows.Values))
	for _, row := range rows.Values {
		out = append(out, Wallet{
			ID:           asInt64(row[0]),
			Name:         asString(row[1]),
			BalanceCents: asInt64(row[2]),
			CreatedAt:    time.UnixMilli(asInt64(row[3])).Local(),
		})
	}
	return out, nil
}

func (s *SQLiteStore) budget(ctx context.Context) (int64, error) {
	rows, err := s.db.Query(ctx, `SELECT value FROM settings WHERE key = 'monthly_budget_cents'`)
	if err != nil {
		return 0, err
	}
	if len(rows.Values) == 0 {
		return defaultBudgetCents, nil
	}
	n, err := strconv.ParseInt(asString(rows.Values[0][0]), 10, 64)
	if err != nil || n <= 0 {
		return defaultBudgetCents, nil
	}
	return n, nil
}

func (s *SQLiteStore) monthStats(ctx context.Context, month time.Time) (MonthStats, error) {
	start, end := monthBounds(month)
	rows, err := s.db.Query(ctx, `SELECT
			COALESCE(SUM(CASE WHEN kind = 'expense' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN kind = 'income' THEN amount_cents ELSE 0 END), 0)
		FROM entries WHERE occurred_at >= ? AND occurred_at < ?`, start, end)
	if err != nil {
		return MonthStats{}, err
	}
	if len(rows.Values) == 0 {
		return MonthStats{}, nil
	}
	return MonthStats{
		ExpenseCents: asInt64(rows.Values[0][0]),
		IncomeCents:  asInt64(rows.Values[0][1]),
	}, nil
}

func (s *SQLiteStore) categories(ctx context.Context, month time.Time) ([]CategoryTotal, error) {
	start, end := monthBounds(month)
	rows, err := s.db.Query(ctx, `SELECT category, SUM(amount_cents) AS total
		FROM entries
		WHERE kind = 'expense' AND occurred_at >= ? AND occurred_at < ?
		GROUP BY category
		ORDER BY total DESC, category ASC`, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]CategoryTotal, 0, len(rows.Values))
	for _, row := range rows.Values {
		out = append(out, CategoryTotal{Category: asString(row[0]), AmountCents: asInt64(row[1])})
	}
	return out, nil
}

func (s *SQLiteStore) entries(ctx context.Context, month time.Time) ([]Entry, error) {
	start, end := monthBounds(month)
	rows, err := s.db.Query(ctx, `SELECT e.id, e.wallet_id, COALESCE(w.name, e.wallet_name),
			e.kind, e.category, e.amount_cents, e.note, e.is_adjustment, e.occurred_at, e.created_at
		FROM entries e
		LEFT JOIN wallets w ON w.id = e.wallet_id
		WHERE e.occurred_at >= ? AND e.occurred_at < ?
		ORDER BY e.occurred_at DESC, e.created_at DESC, e.id DESC`, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows.Values))
	for _, row := range rows.Values {
		out = append(out, Entry{
			ID:          asInt64(row[0]),
			WalletID:    nullableInt64(row[1]),
			WalletName:  asString(row[2]),
			Kind:        Kind(asString(row[3])),
			Category:    asString(row[4]),
			AmountCents: asInt64(row[5]),
			Note:        asString(row[6]),
			Adjustment:  asInt64(row[7]) == 1,
			OccurredAt:  time.UnixMilli(asInt64(row[8])).Local(),
			CreatedAt:   time.UnixMilli(asInt64(row[9])).Local(),
		})
	}
	return out, nil
}

func (s *SQLiteStore) walletEntryCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := s.db.Query(ctx, `SELECT wallet_id, COUNT(*) FROM entries WHERE wallet_id IS NOT NULL GROUP BY wallet_id`)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(rows.Values))
	for _, row := range rows.Values {
		out[nullableInt64(row[0])] = int(asInt64(row[1]))
	}
	return out, nil
}

// AddEntry writes the entry and the balance change in one SQLite
// transaction. It returns the new entry as it was stored.
func (s *SQLiteStore) AddEntry(ctx context.Context, in NewEntry) (Entry, error) {
	in.Category = strings.TrimSpace(in.Category)
	in.Note = strings.TrimSpace(in.Note)
	if in.Kind != KindExpense && in.Kind != KindIncome {
		return Entry{}, fmt.Errorf("unknown entry kind %q", in.Kind)
	}
	if in.AmountCents <= 0 || in.AmountCents > MaxAmountCents {
		return Entry{}, fmt.Errorf("invalid amount %d", in.AmountCents)
	}
	if in.Category == "" {
		return Entry{}, ErrEntryCategory
	}
	if utf8.RuneCountInString(in.Note) > maxNoteRunes {
		return Entry{}, ErrEntryNoteTooLong
	}
	if in.WalletID <= 0 {
		return Entry{}, ErrWalletNotFound
	}

	wallet, err := s.wallet(ctx, in.WalletID)
	if err != nil {
		return Entry{}, err
	}
	now := s.now()
	if in.OccurredAt.IsZero() {
		in.OccurredAt = now
	}
	delta := -in.AmountCents
	if in.Kind == KindIncome {
		delta = in.AmountCents
	}
	result, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `INSERT INTO entries(wallet_id, wallet_name, kind, category, amount_cents, note, is_adjustment, occurred_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
			Args: []any{in.WalletID, wallet.Name, string(in.Kind), in.Category, in.AmountCents, in.Note, in.OccurredAt.UnixMilli(), now.UnixMilli()}},
		{SQL: `UPDATE wallets SET balance_cents = balance_cents + ? WHERE id = ?`,
			Args: []any{delta, in.WalletID}},
	})
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		ID:          result[0].LastInsertID,
		WalletID:    in.WalletID,
		WalletName:  wallet.Name,
		Kind:        in.Kind,
		Category:    in.Category,
		AmountCents: in.AmountCents,
		Note:        in.Note,
		OccurredAt:  in.OccurredAt.Local(),
		CreatedAt:   now.Local(),
	}, nil
}

// DeleteEntry removes the entry and restores the wallet balance in the same
// transaction. A calibration entry is deleted without touching the
// balance: Story 2.3 says deleting it does not roll back the calibration.
func (s *SQLiteStore) DeleteEntry(ctx context.Context, id int64) error {
	_, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `UPDATE wallets SET balance_cents = balance_cents +
			CASE WHEN (SELECT kind FROM entries WHERE id = ?1) = 'expense'
				THEN (SELECT amount_cents FROM entries WHERE id = ?1)
				ELSE -(SELECT amount_cents FROM entries WHERE id = ?1)
			END
			WHERE id = (SELECT wallet_id FROM entries WHERE id = ?1)
			  AND COALESCE((SELECT is_adjustment FROM entries WHERE id = ?1), 0) = 0`,
			Args: []any{id}},
		{SQL: `DELETE FROM entries WHERE id = ?`, Args: []any{id}},
	})
	return err
}

func (s *SQLiteStore) CreateWallet(ctx context.Context, name string, initialCents int64) (Wallet, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Wallet{}, ErrWalletName
	}
	if utf8.RuneCountInString(name) > maxWalletNameRunes {
		return Wallet{}, fmt.Errorf("wallet name is too long")
	}
	if initialCents < 0 {
		return Wallet{}, ErrWalletInitial
	}
	if initialCents > MaxAmountCents {
		return Wallet{}, ErrAmountTooLarge
	}
	now := s.now()
	result, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `INSERT INTO wallets(name, balance_cents, created_at) VALUES (?, ?, ?)`,
			Args: []any{name, initialCents, now.UnixMilli()}},
	})
	if err != nil {
		return Wallet{}, err
	}
	return Wallet{ID: result[0].LastInsertID, Name: name, BalanceCents: initialCents, CreatedAt: now.Local()}, nil
}

func (s *SQLiteStore) RenameWallet(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrWalletName
	}
	if utf8.RuneCountInString(name) > maxWalletNameRunes {
		return fmt.Errorf("wallet name is too long")
	}
	result, err := s.db.Execute(ctx, `UPDATE wallets SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return err
	}
	if result.Changes == 0 {
		return ErrWalletNotFound
	}
	return nil
}

// DeleteWallet removes the wallet. Its entries stay for the history, with
// their wallet name copied, and stop counting toward the wallet total.
func (s *SQLiteStore) DeleteWallet(ctx context.Context, id int64) error {
	result, err := s.db.Execute(ctx, `DELETE FROM wallets WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if result.Changes == 0 {
		return ErrWalletNotFound
	}
	return nil
}

// CalibrateWallet sets a wallet to the balance the user verified and
// records the difference as an adjustment entry. Calibration has its own
// category and is never silently rolled back.
func (s *SQLiteStore) CalibrateWallet(ctx context.Context, id, targetCents int64) (int64, error) {
	if targetCents > MaxAmountCents || targetCents < -MaxAmountCents {
		return 0, ErrAmountTooLarge
	}
	wallet, err := s.wallet(ctx, id)
	if err != nil {
		return 0, err
	}
	diff := targetCents - wallet.BalanceCents
	if diff == 0 {
		return 0, nil
	}
	kind := KindExpense
	amount := -diff
	if diff > 0 {
		kind = KindIncome
		amount = diff
	}
	now := s.now()
	note := fmt.Sprintf("余额校准：%s → %s", FormatCents(wallet.BalanceCents), FormatCents(targetCents))
	_, err = s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `UPDATE wallets SET balance_cents = ? WHERE id = ?`, Args: []any{targetCents, id}},
		{SQL: `INSERT INTO entries(wallet_id, wallet_name, kind, category, amount_cents, note, is_adjustment, occurred_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			Args: []any{id, wallet.Name, string(kind), adjustmentCategory, amount, note, now.UnixMilli(), now.UnixMilli()}},
	})
	if err != nil {
		return 0, err
	}
	return diff, nil
}

func (s *SQLiteStore) SetBudget(ctx context.Context, budgetCents int64) error {
	if budgetCents <= 0 {
		return ErrBudget
	}
	if budgetCents > MaxAmountCents {
		return ErrAmountTooLarge
	}
	_, err := s.db.Execute(ctx, `INSERT INTO settings(key, value) VALUES ('monthly_budget_cents', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, strconv.FormatInt(budgetCents, 10))
	return err
}

func (s *SQLiteStore) wallet(ctx context.Context, id int64) (Wallet, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, balance_cents, created_at FROM wallets WHERE id = ?`, id)
	if err != nil {
		return Wallet{}, err
	}
	if len(rows.Values) == 0 {
		return Wallet{}, ErrWalletNotFound
	}
	row := rows.Values[0]
	return Wallet{
		ID:           asInt64(row[0]),
		Name:         asString(row[1]),
		BalanceCents: asInt64(row[2]),
		CreatedAt:    time.UnixMilli(asInt64(row[3])).Local(),
	}, nil
}

func monthStart(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
}

func monthBounds(month time.Time) (startMillis, endMillis int64) {
	start := monthStart(month)
	end := start.AddDate(0, 1, 0)
	return start.UnixMilli(), end.UnixMilli()
}

// AddMonths steps a month while keeping it normalized to the first day,
// local time.
func AddMonths(t time.Time, delta int) time.Time {
	start := monthStart(t)
	return start.AddDate(0, delta, 0)
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	case nil:
		return 0
	default:
		return 0
	}
}

func nullableInt64(v any) int64 {
	if v == nil {
		return 0
	}
	return asInt64(v)
}

func asString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
