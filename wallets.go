package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (a *App) walletsPage(c *ui.Context) {
	p := lookFor(c)
	ui.Column(c).Fill().Padding(14, 28, 16).Gap(14).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "钱包们").FontSize(22).FontWeight(700).TextColor(p.text)
				ui.Text(c, "每只钱包都有自己的小胃口，余额永远由流水算出来。").FontSize(11).TextColor(p.muted)
			})
			ui.Spacer(c)
			if ui.PrimaryButton(c, "添加钱包").Label("添加钱包").Height(34).Clicked() {
				a.openAddWallet()
			}
		})

		ui.Row(c).Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
			if len(a.snap.Wallets) == 0 {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Column(c).Width(420).Padding(28).Center().Gap(10).Radius(20).
						Background(p.surfaceAlt).Border(1, p.border).Children(func() {
						ui.Text(c, "一只钱包都没有").FontSize(17).FontWeight(700).TextColor(p.text)
						ui.Text(c, "先把要记账的钱包请进来，再开始投喂。").FontSize(11).TextColor(p.muted).
							TextAlign(ui.Center)
						if ui.PrimaryButton(c, "添加钱包").Label("添加第一只钱包").Clicked() {
							a.openAddWallet()
						}
					})
				})
			} else {
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Gap(10).Children(func() {
						for _, w := range a.displayWallets() {
							a.walletManageCard(c, w)
						}
						ui.Text(c, "删除钱包不会删掉历史流水，只是不再计入钱包合计。").
							FontSize(9.5).TextColor(p.muted).Padding(2, 4)
					})
				})
			}
			a.walletsAside(c)
		})
	})
}

func (a *App) walletManageCard(c *ui.Context, w Wallet) {
	p := lookFor(c)
	selected := a.form.walletID == w.ID
	negative := w.BalanceCents < 0
	mood := MoodFor(w.BalanceCents, a.snap.BudgetCents)
	count := a.snap.WalletEntryCount[w.ID]

	card := ui.Column(c).Padding(14, 15).Gap(11).Radius(18).
		Background(p.surface).Border(1, p.border).Transition(ui.ElementTransition{Colors: true})
	if selected {
		card.Border(1, alpha(p.accent, 0.55)).Background(p.surface.Mix(p.accentSoft, 0.45))
	}
	card.Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(38, 38).Radius(12).Background(p.surfaceAlt).Children(func() {
				ui.Text(c, "👛").FontSize(18).Center()
			})
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, w.Name).FontSize(15).FontWeight(650).TextColor(p.text).SingleLine()
				ui.Textf(c, "%d 笔记录 · 创建于 %s", count, w.CreatedAt.Format("2006年1月2日")).
					FontSize(9.5).TextColor(p.muted)
			})
			ui.Column(c).AlignItems(ui.End).Gap(2).Children(func() {
				color := p.text
				if negative {
					color = p.coral
				}
				ui.Text(c, FormatCents(w.BalanceCents)).FontSize(18).FontWeight(700).
					TextColor(color).SingleLine()
				tagColor := moodColor(mood, p)
				tagBG := p.surfaceWarm
				if negative {
					tagColor = p.coral
					tagBG = p.coralSoft
				}
				ui.Text(c, WalletLabel(w.BalanceCents, a.snap.BudgetCents)).
					FontSize(9).FontWeight(600).TextColor(tagColor).
					Padding(2, 7).Radius(999).Background(tagBG)
			})
		})

		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			if selected {
				ui.Text(c, "录入默认").FontSize(9).FontWeight(600).TextColor(p.accent).
					Padding(2, 7).Radius(999).Background(p.accentSoft)
			} else {
				if smallTextButton(c, "设为默认", "设为默认："+w.Name, p).Clicked() {
					a.form.walletID = w.ID
					a.hint = ""
					a.toast = fmt.Sprintf("「%s」成了录入默认钱包", w.Name)
				}
			}
			ui.Spacer(c)
			if smallTextButton(c, "校准", "校准余额："+w.Name, p).Clicked() {
				a.openCalibrateWallet(w)
			}
			if smallTextButton(c, "改名", "重命名："+w.Name, p).Clicked() {
				a.openRenameWallet(w)
			}
			del := smallTextButtonColor(c, "删除", "删除钱包："+w.Name, p, p.coral)
			if del.Clicked() {
				a.askDeleteWallet(w.ID)
			}
		})
	})
}

func smallTextButton(c *ui.Context, label, accessibilityLabel string, p look) ui.Element {
	return smallTextButtonColor(c, label, accessibilityLabel, p, p.muted)
}

func smallTextButtonColor(c *ui.Context, label, accessibilityLabel string, p look, color ui.Color) ui.Element {
	b := ui.ButtonBase(c).Padding(4, 8).Radius(7).Label(accessibilityLabel).Tooltip(accessibilityLabel)
	b.Children(func() {
		ui.Text(c, label).FontSize(10).FontWeight(500).TextColor(color)
	})
	if b.Hovered() {
		b.Background(p.surfaceAlt)
	}
	return b
}

func (a *App) walletsAside(c *ui.Context) {
	p := lookFor(c)
	used := usedPercent(a.snap.CurrentStats.ExpenseCents, a.snap.BudgetCents)
	remain := a.snap.BudgetCents - a.snap.CurrentStats.ExpenseCents
	ui.Column(c).Width(296).Shrink(0).Gap(12).Children(func() {
		ui.Column(c).Padding(16, 17).Gap(9).Radius(18).Background(p.surface).Border(1, p.border).
			Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "月预算").FontSize(12).FontWeight(700).TextColor(p.text)
					ui.Spacer(c)
					ui.Text(c, monthLabelCN(a.currentMonth())).FontSize(9.5).TextColor(p.muted)
				})
				ui.Text(c, FormatCents(a.snap.BudgetCents)).FontSize(24).FontWeight(700).TextColor(p.text)
				ui.Box(c).Height(6).Radius(3).Background(p.surfaceWarm).Children(func() {
					if used > 0 {
						color := p.accent
						if used >= 100 {
							color = p.coral
						} else if used >= 70 {
							color = p.amber
						}
						ui.Box(c).Height(6).Radius(3).WidthPercent(float32(used)).Background(color)
					}
				})
				ui.Row(c).Children(func() {
					ui.Textf(c, "已用 %s", FormatCents(a.snap.CurrentStats.ExpenseCents)).
						FontSize(9.5).TextColor(p.muted)
					ui.Spacer(c)
					remainText := "剩 " + FormatCents(remain)
					if remain < 0 {
						remainText = "超出 " + FormatCents(-remain)
					}
					ui.Text(c, remainText).FontSize(9.5).
						TextColor(map[bool]ui.Color{true: p.coral, false: p.muted}[remain < 0])
				})
				adjust := ui.ButtonBase(c).FillWidth().Height(31).Radius(8).Label("调整月预算").
					Background(p.surfaceAlt).Border(1, p.border)
				adjust.Children(func() {
					ui.Row(c).Fill().Center().Children(func() {
						ui.Text(c, "调整月预算").FontSize(10.5).FontWeight(500).TextColor(p.text)
					})
				})
				if adjust.Hovered() {
					adjust.Background(p.surfaceWarm)
				}
				if adjust.Clicked() {
					a.openBudget()
				}
			})

		ui.Column(c).Grow(1).Padding(16, 17).Gap(9).Radius(18).
			Background(p.surfaceAlt).Border(1, p.border).Children(func() {
			ui.Text(c, "钱包合计").FontSize(11).FontWeight(600).TextColor(p.muted)
			ui.Text(c, FormatCents(a.snap.WalletTotalCents)).FontSize(20).FontWeight(700).
				TextColor(p.text).SingleLine()
			ui.Box(c).Height(1).FillWidth().Background(p.hairline)
			ui.Text(c, "账本是这么算的").FontSize(10.5).FontWeight(600).TextColor(p.text)
			guideLine(c, "🍜", "支出从选中的钱包余额里扣掉。", p)
			guideLine(c, "💼", "收入会把余额加回那只钱包。", p)
			guideLine(c, "🧮", "校准余额会留下一条可查的流水。", p)
			ui.Spacer(c)
			ui.Text(c, "所有数据只在这台电脑上。").FontSize(9).TextColor(p.muted)
		})
	})
}

func guideLine(c *ui.Context, emoji, text string, p look) {
	ui.Row(c).Gap(7).AlignItems(ui.Start).Children(func() {
		ui.Text(c, emoji).FontSize(11)
		ui.Text(c, text).FontSize(9.5).TextColor(p.muted).Grow(1)
	})
}
