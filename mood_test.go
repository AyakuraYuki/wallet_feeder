package main

import (
	"math"
	"testing"
)

func TestMoodBoundaries(t *testing.T) {
	const budget = int64(100_000) // ¥1,000
	tests := []struct {
		balance int64
		want    Mood
	}{
		{50_001, MoodPlump},
		{50_000, MoodOkay},
		{20_000, MoodOkay},
		{19_999, MoodSlim},
		{0, MoodSlim},
		{-1, MoodSlim},
		{math.MaxInt64, MoodPlump},
		{math.MaxInt64/2 + 1, MoodPlump},
	}
	for _, tt := range tests {
		if got := MoodFor(tt.balance, budget); got != tt.want {
			t.Errorf("MoodFor(%d, %d) = %d, want %d", tt.balance, budget, got, tt.want)
		}
	}
}

func TestMoodLabels(t *testing.T) {
	if MoodLabel(MoodPlump) != "鼓鼓的" || MoodLabel(MoodOkay) != "还行" || MoodLabel(MoodSlim) != "瘪瘪的" {
		t.Fatalf("unexpected labels: %q %q %q", MoodLabel(MoodPlump), MoodLabel(MoodOkay), MoodLabel(MoodSlim))
	}
	if WalletLabel(-1, 100_000) != "透支" {
		t.Fatalf("negative wallet label = %q, want 透支", WalletLabel(-1, 100_000))
	}
}

func TestCategoryBreakdown(t *testing.T) {
	in := []CategoryTotal{
		{"餐饮", 500}, {"交通", 400}, {"购物", 300}, {"居住", 200}, {"娱乐", 100}, {"医疗", 80}, {"其他", 50},
	}
	got := categoryBreakdown(in, 1630)
	if len(got) != 6 || got[5].Category != "其他" || got[5].AmountCents != 130 {
		t.Fatalf("breakdown = %+v, want top five and a merged 其他 row of 130", got)
	}

	small := in[:4]
	if got := categoryBreakdown(small, 1400); len(got) != 4 {
		t.Fatalf("small breakdown = %+v, want unchanged", got)
	}

	if got := categoryPercent(50, 200); got != 25 {
		t.Fatalf("percent = %v, want 25", got)
	}
	if got := categoryPercent(1, 0); got != 0 {
		t.Fatalf("empty percent = %v, want 0", got)
	}
}
