package main

// Mood is how full the wallet looks, from the ratio of a balance to the
// monthly budget. Story 1.3 fixes the thresholds: above 50% is plump,
// 20%..50% is okay, below 20% is slim. A negative balance is always slim and
// is shown as overdrawn.
type Mood int

const (
	MoodSlim Mood = iota
	MoodOkay
	MoodPlump
)

// MoodFor classifies a balance against the month's budget.
func MoodFor(balanceCents, budgetCents int64) Mood {
	if balanceCents < 0 {
		return MoodSlim
	}
	if budgetCents <= 0 {
		// A budget is required to be positive; this only guards damaged
		// data. Treat any positive balance as okay rather than full.
		if balanceCents > 0 {
			return MoodOkay
		}
		return MoodSlim
	}
	switch {
	case balanceCents*2 > budgetCents:
		return MoodPlump
	case balanceCents*5 >= budgetCents:
		return MoodOkay
	default:
		return MoodSlim
	}
}

// MoodLabel is the user-facing name of a mood. The home wallet card and the
// wallet list both use it, so the vocabulary stays consistent.
func MoodLabel(m Mood) string {
	switch m {
	case MoodPlump:
		return "鼓鼓的"
	case MoodOkay:
		return "还行"
	default:
		return "瘪瘪的"
	}
}

// MoodBlurb is the wallet's one-line reaction to its state.
func MoodBlurb(m Mood) string {
	switch m {
	case MoodPlump:
		return "吃得饱饱的，走路都带风。"
	case MoodOkay:
		return "不胖不瘦，刚刚好。"
	default:
		return "肚子有点扁，下口悠着点。"
	}
}

// WalletLabel is the label shown for one wallet. A negative wallet is
// called "透支" instead of its mood name, as Story 3.3 asks.
func WalletLabel(balanceCents, budgetCents int64) string {
	if balanceCents < 0 {
		return "透支"
	}
	return MoodLabel(MoodFor(balanceCents, budgetCents))
}

// Fullness returns a 0..1 measure for drawing the wallet's silhouette.
// Keeping the bands wider than 0 is what makes the drawing read as a body
// rather than a dot.
func Fullness(balanceCents, budgetCents int64) float32 {
	if budgetCents <= 0 {
		budgetCents = 1
	}
	r := float32(balanceCents) / float32(budgetCents)
	if r < 0 {
		r = 0
	}
	if r > 1.25 {
		r = 1.25
	}
	return 0.35 + 0.55*(r/1.25)
}
