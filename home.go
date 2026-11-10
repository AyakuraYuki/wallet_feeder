package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *App) homePage(c *ui.Context) {
	ui.Column(c).Fill().Padding(14, 28, 16).Gap(14).Children(func() {
		a.summaryStrip(c)
		ui.Row(c).Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Width(378).Shrink(0).Gap(14).Children(func() {
				a.walletStage(c)
				a.walletsCard(c)
			})
			ui.Column(c).Grow(1).Gap(14).Children(func() {
				if len(a.breakdown) == 0 {
					a.feedCard(c, true)
				} else {
					a.feedCard(c, false)
					a.categoryCard(c).Grow(1)
				}
			})
		})
	})
}

func (a *App) summaryStrip(c *ui.Context) {
	p := lookFor(c)
	usedPercent := 0.0
	if a.snap.BudgetCents > 0 {
		usedPercent = clampPercent(float64(a.snap.CurrentStats.ExpenseCents) / float64(a.snap.BudgetCents) * 100)
	}
	spendNote := "这个月还没有支出"
	if a.snap.CurrentStats.ExpenseCents > 0 {
		spendNote = fmt.Sprintf("月预算已用 %.0f%%", usedPercent)
	}
	incomeNote := "本月钱包进账"
	if a.snap.CurrentStats.IncomeCents == 0 {
		incomeNote = "还没有收入记录"
	}
	walletNote := fmt.Sprintf("共 %d 只钱包", len(a.snap.Wallets))
	if a.snap.WalletTotalCents < 0 {
		walletNote = "有钱包在透支"
	}
	ui.Row(c).Height(88).Gap(12).Children(func() {
		statCard(c, "本月支出", FormatCents(a.snap.CurrentStats.ExpenseCents), spendNote, p.coral)
		statCard(c, "本月收入", FormatCents(a.snap.CurrentStats.IncomeCents), incomeNote, p.mint)
		statCard(c, "钱包合计", FormatCents(a.snap.WalletTotalCents), walletNote,
			moodColor(MoodFor(a.snap.WalletTotalCents, a.snap.BudgetCents), p))
	})
}

func statCard(c *ui.Context, label, value, note string, accent ui.Color) {
	p := lookFor(c)
	ui.Column(c).Grow(1).FillHeight().Padding(13, 16).Radius(16).
		Background(p.surface).Border(1, p.border).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(accent)
			ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(p.muted)
		})
		ui.Text(c, value).FontSize(23).FontWeight(700).TextColor(p.text).SingleLine()
		ui.Text(c, note).FontSize(10).TextColor(p.muted).SingleLine()
	})
}

func (a *App) walletStage(c *ui.Context) {
	p := lookFor(c)
	mood := MoodFor(a.snap.WalletTotalCents, a.snap.BudgetCents)
	negative := a.snap.WalletTotalCents < 0
	pal := stagePalette(mood, negative, p)
	reaction := a.activeReaction(c)
	reactionKind := KindExpense
	if reaction != nil {
		reactionKind = reaction.kind
	}
	titleColor := p.text
	if !c.Theme().Dark {
		titleColor = pal.face
	}
	if negative {
		titleColor = p.coral
	}

	card := ui.Column(c).Grow(1).Padding(16, 16).Gap(6).Radius(22).Clip().
		Border(1, p.border)
	card.Gradient(pal.top, pal.bottom, 155)
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(5, 5).Radius(2.5).Background(pal.outline)
				ui.Text(c, "钱包心情").FontSize(10.5).FontWeight(600).TextColor(p.muted)
			})
			ui.Spacer(c)
			budget := ui.ButtonBase(c).Padding(5, 9).Radius(999).
				Background(alpha(p.surface, 0.62)).Label("调整月预算").Tooltip("调整月预算")
			budget.Children(func() {
				ui.Textf(c, "月预算 %s", FormatCentsCompact(a.snap.BudgetCents)).
					FontSize(10).FontWeight(500).TextColor(p.muted)
			})
			if budget.Clicked() {
				a.openBudget()
			}
			if budget.Hovered() {
				budget.Background(alpha(p.surface, 0.9))
			}
		})

		ui.Box(c).Grow(1).MinHeight(120).Draw(func(pt *ui.Painter, r ui.Rect) {
			drawWalletCreature(pt, r, mood, negative, Fullness(a.snap.WalletTotalCents, a.snap.BudgetCents),
				reaction != nil, reactionKind, p)
		})

		if reaction != nil {
			title, line := "被喂了一口！", ""
			if reaction.kind == KindIncome {
				title = "钱包鼓了一点！"
				line = fmt.Sprintf("「%s」收进了 %s", reaction.walletName, FormatCents(reaction.amountCents))
			} else {
				line = fmt.Sprintf("「%s」吃下了 %s", reaction.walletName, FormatCents(reaction.amountCents))
			}
			ui.Text(c, title).FontSize(21).FontWeight(700).TextColor(titleColor)
			ui.Text(c, line).FontSize(11.5).TextColor(p.muted)
		} else {
			ui.Text(c, WalletLabel(a.snap.WalletTotalCents, a.snap.BudgetCents)).
				FontSize(22).FontWeight(700).TextColor(titleColor)
			ui.Text(c, MoodBlurb(mood)).FontSize(11.5).TextColor(p.muted)
		}

		if a.snap.CurrentStats.ExpenseCents == 0 && a.snap.CurrentStats.IncomeCents == 0 {
			ui.Text(c, "还没投喂过，从第一笔开始吧").FontSize(11).
				FontWeight(600).TextColor(p.accent).Padding(2, 0, 0, 0)
		}

		// Budget consumption: the goal is to make the wallet feel fed, not
		// to police spending. It stays a quiet, information-only strip.
		used := 0.0
		if a.snap.BudgetCents > 0 {
			used = clampPercent(float64(a.snap.CurrentStats.ExpenseCents) / float64(a.snap.BudgetCents) * 100)
		}
		barColor := p.accent
		if used >= 100 {
			barColor = p.coral
		} else if used >= 70 {
			barColor = p.amber
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "本月已喂 %s", FormatCents(a.snap.CurrentStats.ExpenseCents)).
				FontSize(10).TextColor(p.muted)
			ui.Spacer(c)
			ui.Textf(c, "预算 %s", FormatCentsCompact(a.snap.BudgetCents)).
				FontSize(10).TextColor(p.muted)
		})
		ui.Box(c).Height(5).Radius(3).Background(alpha(p.surface, 0.62)).Children(func() {
			if used > 0 {
				ui.Box(c).Height(5).Radius(3).WidthPercent(float32(used)).Background(barColor)
			}
		})
	})
}

func (a *App) walletsCard(c *ui.Context) {
	p := lookFor(c)
	card := ui.Column(c).Height(218).Padding(12, 14).Gap(7).Radius(20).
		Background(p.surface).Border(1, p.border)
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "钱包们").FontSize(13.5).FontWeight(700).TextColor(p.text)
			ui.Textf(c, " %d", len(a.snap.Wallets)).FontSize(10).TextColor(p.muted)
			ui.Spacer(c)
			manage := ui.ButtonBase(c).Padding(4, 7).Radius(7).Label("管理钱包")
			manage.Children(func() {
				ui.Text(c, "管理").FontSize(10.5).TextColor(p.muted)
			})
			if manage.Hovered() {
				manage.Background(p.surfaceAlt)
			}
			if manage.Clicked() {
				a.setPage(pageWallets)
			}
			add := ui.ButtonBase(c).Padding(4, 7).Radius(7).Label("添加钱包")
			add.Children(func() {
				ui.Text(c, "＋").FontSize(14).FontWeight(600).TextColor(p.accent)
			})
			if add.Hovered() {
				add.Background(p.accentSoft)
			}
			if add.Clicked() {
				a.openAddWallet()
			}
		})

		wallets := a.displayWallets()
		if len(wallets) == 0 {
			ui.Column(c).Grow(1).Center().Gap(5).Children(func() {
				ui.Text(c, "这里空荡荡的").FontSize(12).FontWeight(600).TextColor(p.text)
				if ui.Button(c, "添加第一只钱包").Label("添加第一只钱包").Clicked() {
					a.openAddWallet()
				}
			})
			return
		}
		a.walletList.Key = func(i int) any { return wallets[i].ID }
		list := ui.List(c, &a.walletList, len(wallets), func(i int) {
			a.walletStatusRow(c, wallets[i], p)
		}).Grow(1).Gap(3)
		_ = list
	})
}

func (a *App) walletStatusRow(c *ui.Context, w Wallet, p look) {
	selected := a.form.walletID == w.ID
	negative := w.BalanceCents < 0
	mood := MoodFor(w.BalanceCents, a.snap.BudgetCents)
	b := ui.ButtonBase(c).Padding(4, 8).Height(31).Radius(9).
		Label("选择钱包："+w.Name).Tooltip("设为默认钱包").Border(1, ui.Transparent)
	if selected {
		b.Background(p.accentSoft).Border(1, alpha(p.accent, 0.45))
	} else if b.Hovered() {
		b.Background(p.surfaceAlt)
	}
	b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			dot := p.mint
			if negative {
				dot = p.coral
			} else {
				dot = moodColor(mood, p)
			}
			ui.Box(c).Size(5, 5).Radius(2.5).Background(dot)
			ui.Text(c, w.Name).FontSize(10.5).FontWeight(600).TextColor(p.text).SingleLine().Grow(1)
			if selected {
				ui.Text(c, "录入").FontSize(8).FontWeight(600).TextColor(p.accent)
			}
			color := p.text
			if negative {
				color = p.coral
			}
			ui.Text(c, FormatCentsCompact(w.BalanceCents)).FontSize(10.5).
				FontWeight(600).TextColor(color)
			tagColor := moodColor(mood, p)
			tagBG := p.surfaceWarm
			if negative {
				tagColor = p.coral
				tagBG = p.coralSoft
			}
			ui.Text(c, WalletLabel(w.BalanceCents, a.snap.BudgetCents)).
				FontSize(8).FontWeight(500).TextColor(tagColor).
				Padding(1, 5).Radius(999).Background(tagBG)
		})
	})
	if b.Clicked() {
		a.form.walletID = w.ID
		a.hint = ""
		a.shakeWalletUntil = time.Time{}
		a.focusAmount = true
	}
}

func (a *App) feedCard(c *ui.Context, grow bool) {
	p := lookFor(c)
	card := ui.Column(c).Padding(16, 18).Gap(7).Radius(20).
		Background(p.surface).Border(1, p.border)
	if grow {
		card.Grow(1)
	} else {
		card.Height(350)
	}
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "喂一口").FontSize(18).FontWeight(700).TextColor(p.text)
				ui.Text(c, "金额 → 分类 → 钱包 → 回车").FontSize(9.5).TextColor(p.muted)
			})
			ui.Spacer(c)
			a.kindToggle(c)
		})

		// Amount: the single ui.TextInput of the quick-entry path, bound to
		// a handle so the form can focus it after each saved bite.
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "¥").FontSize(24).FontWeight(700).TextColor(p.muted).Padding(0, 0, 0, 2)
			in := ui.TextInput(c.Key("entry-amount"), &a.form.amount).
				Bind(&a.amountRef).Grow(1).Height(48).FontSize(25).FontWeight(650).
				Label("金额").Placeholder("0.00").MaxLength(16).Disabled(a.busy).AutoFocus()
			if a.amountInvalidMessage() != "" {
				in.Invalid(true)
			}
			if in.Changed() {
				a.hint = ""
			}
			if in.Submitted() && !a.busy {
				a.submitEntry(c)
			}
		})

		// Categories.
		catOffset := shakeOffset(c, &a.shakeCategoryUntil)
		catBad := catOffset != 0
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "分类").FontSize(10).TextColor(p.muted)
			if catBad {
				ui.Text(c, "选一个").FontSize(9.5).FontWeight(600).TextColor(p.coral)
			}
		})
		ui.Row(c).Gap(5).Wrap().Translate(catOffset, 0).Children(func() {
			for _, cat := range categoriesFor(a.form.kind) {
				selected := a.form.category == cat.Name
				chip := ui.ButtonBase(c).Padding(5, 9).Height(31).Radius(999).
					Label("分类：" + cat.Name).Disabled(a.busy)
				switch {
				case selected:
					chip.Background(p.accent)
				case catBad:
					chip.Background(p.coralSoft).Border(1, p.coral)
				default:
					chip.Background(p.background).Border(1, p.border)
				}
				if chip.Hovered() && !selected {
					chip.Background(p.surfaceAlt)
				}
				chip.Children(func() {
					color := p.text
					if selected {
						color = p.surface
					}
					ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Text(c, cat.Emoji).FontSize(12)
						ui.Text(c, cat.Name).FontSize(10.5).FontWeight(500).TextColor(color)
					})
				})
				if chip.Clicked() && !a.busy {
					a.form.category = cat.Name
					a.hint = ""
					a.shakeCategoryUntil = time.Time{}
					a.focusAmount = true
				}
			}
		})

		// Wallets.
		walOffset := shakeOffset(c, &a.shakeWalletUntil)
		walBad := walOffset != 0
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "钱包").FontSize(10).TextColor(p.muted)
			if walBad {
				ui.Text(c, "选一只").FontSize(9.5).FontWeight(600).TextColor(p.coral)
			}
		})
		ui.Row(c).Gap(5).Wrap().Translate(walOffset, 0).Children(func() {
			if len(a.snap.Wallets) == 0 {
				ui.Text(c, "还没有钱包，去「钱包」页添加一只吧").FontSize(10.5).TextColor(p.coral)
				return
			}
			for _, w := range a.snap.Wallets {
				selected := a.form.walletID == w.ID
				chip := ui.ButtonBase(c).Padding(5, 9).Height(31).Radius(999).
					Label("钱包：" + w.Name).Disabled(a.busy)
				switch {
				case selected:
					chip.Background(p.text)
				case walBad:
					chip.Background(p.coralSoft).Border(1, p.coral)
				default:
					chip.Background(p.background).Border(1, p.border)
				}
				if chip.Hovered() && !selected {
					chip.Background(p.surfaceAlt)
				}
				chip.Children(func() {
					color := p.text
					if selected {
						color = p.surface
					}
					ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "👛").FontSize(11)
						ui.Text(c, w.Name).FontSize(10.5).FontWeight(500).TextColor(color).SingleLine()
					})
				})
				if chip.Clicked() && !a.busy {
					a.form.walletID = w.ID
					a.hint = ""
					a.shakeWalletUntil = time.Time{}
					a.focusAmount = true
				}
			}
		})

		ui.Spacer(c)
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			note := ui.TextInput(c.Key("entry-note"), &a.form.note).
				Grow(1).Height(38).FontSize(11.5).Label("备注").
				Placeholder("备注（选填）").MaxLength(maxNoteRunes).Disabled(a.busy)
			if note.Submitted() && !a.busy {
				a.submitEntry(c)
			}
			_, amountOK := a.amountCents()
			feed := ui.PrimaryButton(c, "投喂").Label("投喂钱包").Height(38).Padding(0, 20)
			feed.Disabled(!amountOK || a.busy)
			if feed.Clicked() {
				a.submitEntry(c)
			}
		})

		hint := a.hint
		if hint == "" {
			hint = a.amountInvalidMessage()
		}
		if a.busy {
			hint = "正在记下这一笔…"
		}
		ui.Box(c).Height(15).Children(func() {
			if hint != "" {
				ui.Text(c, hint).FontSize(10).TextColor(p.coral).SingleLine()
			}
		})

		if a.focusAmount {
			a.amountRef.Focus()
			a.focusAmount = false
		}

		if grow && len(a.breakdown) == 0 {
			ui.Spacer(c)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(10, 11).Radius(12).
				Background(p.surfaceAlt).Children(func() {
				ui.Text(c, "🌱").FontSize(17)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "第一口会让这里长出口味地图").FontSize(10.5).FontWeight(600).TextColor(p.text)
					ui.Text(c, "分类占比会在有了支出之后出现。").FontSize(9.5).TextColor(p.muted)
				})
			})
		}
	})
}

func (a *App) kindToggle(c *ui.Context) {
	p := lookFor(c)
	ui.Row(c).Padding(3).Gap(2).Radius(999).Background(p.background).Border(1, p.border).Children(func() {
		for i, label := range []string{"支出", "收入"} {
			kind := KindExpense
			if i == 1 {
				kind = KindIncome
			}
			selected := a.form.kind == kind
			b := ui.ButtonBase(c).Padding(4, 10).Radius(999).Label("记录" + label).Disabled(a.busy)
			if selected {
				b.Background(p.surface).Shadow(0, 1, 2, 0, p.shadow)
			}
			b.Children(func() {
				color := p.muted
				if selected {
					color = p.text
				}
				ui.Text(c, label).FontSize(10.5).FontWeight(600).TextColor(color)
			})
			if b.Clicked() {
				a.setKind(kind)
			}
		}
	})
}

func (a *App) categoryCard(c *ui.Context) ui.Element {
	p := lookFor(c)
	card := ui.Column(c).Padding(13, 16).Gap(5).Radius(20).
		Background(p.surface).Border(1, p.border)
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
			ui.Text(c, "这个月喂去了哪里").FontSize(13.5).FontWeight(700).TextColor(p.text)
			ui.Spacer(c)
			ui.Textf(c, "合计 %s", FormatCents(a.snap.CurrentStats.ExpenseCents)).
				FontSize(10).TextColor(p.muted)
		})
		for i, cat := range a.breakdown {
			if i > 0 {
				ui.Box(c).Height(4)
			}
			pct := categoryPercent(cat.AmountCents, a.snap.CurrentStats.ExpenseCents)
			color := categoryColor(cat.Category, p)
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, categoryEmoji(cat.Category)).FontSize(11)
				ui.Text(c, cat.Category).FontSize(10.5).FontWeight(500).TextColor(p.text)
				ui.Spacer(c)
				ui.Text(c, FormatCents(cat.AmountCents)).FontSize(10.5).FontWeight(600).TextColor(p.text)
				ui.Textf(c, "%.1f%%", pct).FontSize(9.5).TextColor(p.muted).Width(42).TextAlign(ui.End)
			})
			ui.Box(c).Height(3.5).Radius(2).Background(p.surfaceWarm).Children(func() {
				if pct > 0 {
					ui.Box(c).Height(3.5).Radius(2).WidthPercent(float32(pct)).Background(color)
				}
			})
		}
	})
	return card
}

func categoryColor(name string, p look) ui.Color {
	switch name {
	case "餐饮":
		return p.coral
	case "交通":
		return p.blue
	case "购物":
		return p.plum
	case "居住":
		return p.amber
	case "娱乐":
		return p.mint
	case adjustmentCategory:
		return p.muted
	default:
		return p.borderStrong
	}
}

func moodColor(m Mood, p look) ui.Color {
	switch m {
	case MoodPlump:
		return p.mint
	case MoodOkay:
		return p.amber
	default:
		return p.coral
	}
}

func usedPercent(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return math.Min(100, float64(part)/float64(whole)*100)
}
