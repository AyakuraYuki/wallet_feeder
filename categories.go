package main

// Kind distinguishes money that leaves a wallet from money that arrives.
type Kind string

const (
	KindExpense Kind = "expense"
	KindIncome  Kind = "income"
)

// Category is one choosable food or source of money.
type Category struct {
	Name  string
	Emoji string
}

// Expense categories are deliberately short: the entry form should fit on
// one row. The first ones are food, as the product's "feed the wallet"
// language suggests.
var expenseCategories = []Category{
	{Name: "餐饮", Emoji: "🍜"},
	{Name: "交通", Emoji: "🚇"},
	{Name: "购物", Emoji: "🛍️"},
	{Name: "居住", Emoji: "🏠"},
	{Name: "娱乐", Emoji: "🎮"},
	{Name: "其他", Emoji: "✨"},
}

// Income categories are the four sources in Story 1.2.
var incomeCategories = []Category{
	{Name: "工资", Emoji: "💼"},
	{Name: "兼职", Emoji: "🛠️"},
	{Name: "红包", Emoji: "🧧"},
	{Name: "其他", Emoji: "💰"},
}

const adjustmentCategory = "余额校准"

func categoriesFor(kind Kind) []Category {
	if kind == KindIncome {
		return incomeCategories
	}
	return expenseCategories
}

func categoryEmoji(name string) string {
	for _, list := range [][]Category{expenseCategories, incomeCategories} {
		for _, c := range list {
			if c.Name == name {
				return c.Emoji
			}
		}
	}
	if name == adjustmentCategory {
		return "🧮"
	}
	return "🧾"
}

// CategoryTotal is one row of the monthly spending breakdown.
type CategoryTotal struct {
	Category    string
	AmountCents int64
}

// categoryBreakdown merges everything after the fifth category into an
// "其他" row. The real "其他" category is folded into that row too, so the
// chart never shows two rows with the same name. Percentages use the total
// of the month as their denominator, as Story 3.2 says.
func categoryBreakdown(totals []CategoryTotal, totalExpenseCents int64) []CategoryTotal {
	if len(totals) <= 5 {
		return totals
	}
	var kept []CategoryTotal
	var other int64
	for _, t := range totals {
		if t.Category == "其他" {
			other += t.AmountCents
			continue
		}
		if len(kept) < 5 {
			kept = append(kept, t)
			continue
		}
		other += t.AmountCents
	}
	if other > 0 {
		kept = append(kept, CategoryTotal{Category: "其他", AmountCents: other})
	}
	return kept
}

// categoryPercent formats the percentage for a category row. The caller
// rounds to one decimal in the view.
func categoryPercent(amountCents, totalCents int64) float64 {
	if totalCents <= 0 {
		return 0
	}
	return clampPercent(float64(amountCents) / float64(totalCents) * 100)
}
