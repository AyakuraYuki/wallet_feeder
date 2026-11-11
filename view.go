package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// view is the whole window, rebuilt from state on every frame.
func (a *App) view(c *ui.Context) {
	a.applyTheme(c)
	p := lookFor(c)
	a.shortcuts(c)
	a.refreshAtMonthBoundary(c)

	root := ui.Column(c).Fill()
	if a.useGlass {
		root.Draw(a.drawLiquidBackdrop)
	} else {
		root.Background(p.background)
	}
	root.Children(func() {
		a.header(c)
		if a.ready && a.refreshNeeded {
			a.refreshBanner(c)
		}
		switch {
		case a.fatalErr != "":
			a.errorPage(c)
		case !a.ready:
			a.loadingPage(c)
		default:
			switch a.page {
			case pageLedger:
				a.ledgerPage(c)
			case pageWallets:
				a.walletsPage(c)
			default:
				a.homePage(c)
			}
		}
	})

	a.modals(c)
	a.alerts(c)
	a.flushToast(c)
}

func (a *App) shortcuts(c *ui.Context) {
	if a.modalOpen || a.deleteEntryOpen || a.deleteWalletOpen {
		return
	}
	if c.Shortcut(ui.Cmd, ui.Key1) {
		a.setPage(pageHome)
	}
	if c.Shortcut(ui.Cmd, ui.Key2) {
		a.setPage(pageLedger)
	}
	if c.Shortcut(ui.Cmd, ui.Key3) {
		a.setPage(pageWallets)
	}
}

func (a *App) header(c *ui.Context) {
	p := lookFor(c)
	left := float32(28)
	if bar := c.TitleBar(); bar.Left > 0 {
		left = bar.Left + 15
	}
	ui.Row(c).Height(58).FillWidth().Padding(0, 28, 0, left).AlignItems(ui.Center).
		DragWindow().BorderWidth(0, 0, 1, 0).BorderColor(p.hairline).Children(func() {
		ui.Row(c).Gap(11).AlignItems(ui.Center).Children(func() {
			mark := ui.Box(c).Size(38, 38).Radius(12)
			if a.useGlass {
				mark.Material(glass.Glass{Tint: p.accent.Alpha(.18), Interactive: false}).
					Border(1, ui.RGBA(255, 255, 255, .36))
			} else {
				mark.Background(p.accentSoft)
			}
			mark.Draw(func(pt *ui.Painter, r ui.Rect) { drawBrandMark(pt, r, p) })
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "钱包饲养员").FontSize(18).FontWeight(700).TextColor(p.text)
				ui.Text(c, "喂一口，记一笔 · 仅存本地").FontSize(10.5).TextColor(p.muted)
			})
		})
		ui.Spacer(c)
		a.nav(c)
		ui.Spacer(c)
		a.appearanceSwitch(c)
	})
}

// appearanceSwitch is the small demonstration control: one click swaps
// between MyGo's Liquid Glass and the original paper UI. It is itself a
// glass surface in the glass look, with an interactive tinted segment.
func (a *App) appearanceSwitch(c *ui.Context) {
	p := lookFor(c)
	track := ui.Row(c).Gap(2).Padding(2).Radius(999)
	if a.useGlass {
		track.Material(glass.Glass{Clarity: .4})
	} else {
		track.Background(p.surfaceAlt).Border(1, p.border)
	}
	track.Children(func() {
		for i, label := range []string{"极光", "纸感"} {
			glassChoice := i == 0
			selected := a.useGlass == glassChoice
			b := ui.ButtonBase(c).Padding(4, 10).Radius(999).Height(24).
				Label(label + "外观").Tooltip("切换为" + label + "外观")
			switch {
			case selected && a.useGlass:
				b.Material(glass.Glass{Tint: p.accent.Alpha(.80), Interactive: true})
			case selected:
				b.Background(p.surface).Border(1, p.border)
			case b.Hovered():
				if a.useGlass {
					b.Background(ui.RGBA(255, 255, 255, 0.22))
				} else {
					b.Background(p.surfaceWarm)
				}
			}
			b.Children(func() {
				color := p.muted
				if selected {
					color = p.text
					if a.useGlass && glassChoice {
						color = p.surface
					}
				}
				ui.Text(c, label).FontSize(10).FontWeight(600).TextColor(color)
			})
			if b.Clicked() {
				a.useGlass = glassChoice
				a.focusAmount = false
			}
		}
	})
}

func (a *App) nav(c *ui.Context) {
	p := lookFor(c)
	track := ui.Row(c).Gap(2).Padding(3).Radius(999)
	if a.useGlass {
		track.Material(glass.Glass{Clarity: .35}).Border(1, ui.RGBA(255, 255, 255, .28))
	} else {
		track.Background(p.surfaceAlt)
	}
	track.Children(func() {
		for i, label := range []string{"首页", "流水", "钱包"} {
			page := appPage(i)
			selected := a.page == page
			b := ui.ButtonBase(c).Padding(8, 15).Radius(999).Label(label)
			switch {
			case selected && a.useGlass:
				b.Material(glass.Glass{Tint: p.accent.Alpha(.80), Interactive: true})
			case selected:
				b.Background(p.surface).Shadow(0, 1, 3, 0, p.shadow)
			case b.Hovered():
				if a.useGlass {
					b.Background(alpha(p.surface, .20))
				} else {
					b.Background(p.surfaceWarm)
				}
			}
			b.Children(func() {
				color := p.muted
				if selected {
					color = p.text
					if a.useGlass {
						color = c.Theme().AccentText
					}
				}
				ui.Text(c, label).FontSize(12.5).FontWeight(600).TextColor(color)
			})
			if b.Clicked() {
				a.setPage(page)
			}
		}
	})
}

// refreshBanner keeps a read failure visible after its toast disappears.
// Its action only reloads data and can never repeat a committed mutation.
func (a *App) refreshBanner(c *ui.Context) {
	p := lookFor(c)
	ui.Row(c).Height(38).FillWidth().Padding(0, 28).Gap(10).AlignItems(ui.Center).
		Background(p.surfaceAlt).Children(func() {
		ui.Text(c, "账本刷新失败，当前显示最近一次成功读取的数据。").FontSize(11).TextColor(p.coral)
		ui.Spacer(c)
		if ui.Button(c, "刷新账本").Label("刷新账本").Disabled(a.busy || a.refreshing).Clicked() {
			a.reload()
		}
	})
}

func (a *App) loadingPage(c *ui.Context) {
	p := lookFor(c)
	ui.Column(c).Fill().Center().Gap(10).Children(func() {
		ui.Box(c).Size(54, 54).Radius(18).Background(p.accentSoft).
			Draw(func(pt *ui.Painter, r ui.Rect) { drawBrandMark(pt, r, p) })
		ui.Text(c, "钱包正在醒来…").FontSize(17).FontWeight(600).TextColor(p.text)
		ui.Text(c, "正在打开本地账本").FontSize(12).TextColor(p.muted)
	})
}

func (a *App) errorPage(c *ui.Context) {
	p := lookFor(c)
	ui.Column(c).Fill().Center().Padding(24).Children(func() {
		card := a.panel(c, ui.Column(c).Width(430).Padding(24).Gap(12), 20)
		if !a.useGlass {
			card.Shadow(0, 12, 30, -12, p.shadow)
		}
		card.Children(func() {
			ui.Text(c, "钱包睡着了").FontSize(20).FontWeight(700).TextColor(p.text)
			ui.Text(c, a.fatalErr).FontSize(12.5).TextColor(p.muted)
			ui.Text(c, "数据只保存在这台电脑上，重试不会影响已有记录。").FontSize(11.5).TextColor(p.muted)
			ui.Row(c).Gap(8).Children(func() {
				retry := ui.PrimaryButton(c, "再试一次").Label("再试一次")
				if a.useGlass {
					retry.Material(glass.Glass{Tint: p.accent.Alpha(.80), Interactive: true})
				}
				if retry.Clicked() {
					a.retry()
				}
			})
		})
	})
}

func (a *App) flushToast(c *ui.Context) {
	if a.toast == "" {
		return
	}
	c.Toast(a.toast)
	a.toast = ""
}

func (a *App) modals(c *ui.Context) {
	if !a.modalOpen {
		return
	}
	p := lookFor(c)
	// DialogBase dismisses on Escape and backdrop clicks. While a mutation
	// is in flight that would hide both its result and the preserved draft,
	// so acknowledge dismissal only when the dialog is idle.
	open := a.modalOpen
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		if a.useGlass {
			backdrop.Background(ui.RGBA(20, 24, 20, .34))
			panel.Width(440).Padding(22).Gap(10).Radius(26).
				Material(glass.Glass{Tint: p.background.Alpha(.78), Clarity: .05}).
				Border(1, ui.RGBA(255, 255, 255, .42)).
				Shadow(0, 24, 80, -32, ui.RGBA(35, 25, 15, .38))
		} else {
			backdrop.Background(ui.RGBA(0, 0, 0, .4))
			panel.Width(440).Padding(22).Gap(10).Radius(20).
				Background(p.surface).Border(1, p.border).
				Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, .3))
		}
		a.modalContent(c)
	})
	if !open && !a.busy {
		a.closeModal()
	}
}

func (a *App) modalContent(c *ui.Context) {
	p := lookFor(c)
	title, subtitle := "", ""
	switch a.modalKind {
	case modalBudget:
		title = "这个月，喂多少"
		subtitle = "月预算决定钱包的饱腹感。只影响表情和状态，不会限制记账。"
	case modalAddWallet:
		title = "添一只新钱包"
		subtitle = "现金、银行卡、支付宝……每只钱包都有自己的胃口。"
	case modalRename:
		title = "给钱包改名"
		subtitle = "名字会同步显示在流水里。"
	case modalCalibrate:
		title = "校准余额"
		subtitle = "输入钱包里的实际余额，差额会自动记一笔「余额校准」。"
	}
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, title).FontSize(19).FontWeight(700).TextColor(p.text)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(12).TextColor(p.muted)
		}
		if a.modalKind == modalRename || a.modalKind == modalCalibrate {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(8, 10).Radius(9).
				Background(p.surfaceAlt).Children(func() {
				ui.Text(c, "👛").FontSize(14)
				ui.Text(c, a.modalName).FontSize(12).FontWeight(600).TextColor(p.text)
				if a.modalKind == modalCalibrate {
					if w, ok := a.walletByID(a.modalTarget); ok {
						ui.Spacer(c)
						ui.Textf(c, "当前 %s", FormatCents(w.BalanceCents)).
							FontSize(11.5).TextColor(p.muted)
					}
				}
			})
		}
		submitted := false
		if a.modalKind == modalAddWallet || a.modalKind == modalRename {
			ui.Field(c, "钱包名字", func() {
				in := ui.TextInput(c.Key("modal-name"), &a.modalName).
					Placeholder("例如：现金").
					MaxLength(maxWalletNameRunes).Disabled(a.busy)
				if a.modalKind == modalAddWallet || a.modalKind == modalRename {
					in.AutoFocus()
				}
				if in.Submitted() {
					submitted = true
				}
			})
		}
		if a.modalKind == modalAddWallet || a.modalKind == modalCalibrate || a.modalKind == modalBudget {
			label := "初始余额"
			placeholder := "0.00"
			if a.modalKind == modalCalibrate {
				label = "实际余额"
				placeholder = "-0.00"
			}
			if a.modalKind == modalBudget {
				label = "月预算金额"
				placeholder = "3000.00"
			}
			ui.Field(c, label, func() {
				amount := ui.TextInput(c.Key("modal-amount"), &a.modalAmount).
					Placeholder(placeholder).MaxLength(18).Disabled(a.busy)
				if a.modalKind == modalCalibrate || a.modalKind == modalBudget {
					amount.AutoFocus()
				}
				if amount.Submitted() {
					submitted = true
				}
			})
		}
		if a.modalError != "" {
			ui.Text(c, a.modalError).FontSize(11.5).TextColor(p.coral)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Disabled(a.busy).Clicked() {
				a.closeModal()
			}
			action, accessible := "保存", "确认保存"
			switch a.modalKind {
			case modalAddWallet:
				action, accessible = "添加钱包", "确认添加钱包"
			case modalCalibrate:
				action, accessible = "校准", "确认校准余额"
			case modalBudget:
				action, accessible = "更新预算", "确认更新预算"
			}
			if ui.PrimaryButton(c, action).Label(accessible).Disabled(a.busy).Clicked() {
				submitted = true
			}
		})
		if submitted && !a.busy {
			a.submitModal()
		}
	})
}

func (a *App) alerts(c *ui.Context) {
	if a.deleteEntryOpen {
		entry, ok := a.entryByID(a.deleteEntryID)
		if !ok {
			a.deleteEntryOpen = false
		} else {
			message := ""
			if entry.Adjustment {
				message = "这是一条余额校准流水。删除后余额不会自动回滚，钱包会保持刚刚校准后的余额。"
			} else {
				switch {
				case entry.WalletID != 0 && entry.Kind == KindExpense:
					message = fmt.Sprintf("删除后，「%s」的余额会加回 %s。", entry.WalletName, FormatCents(entry.AmountCents))
				case entry.WalletID != 0 && entry.Kind == KindIncome:
					message = fmt.Sprintf("删除后，「%s」的余额会减少 %s。", entry.WalletName, FormatCents(entry.AmountCents))
				default:
					message = "它关联的钱包已经删除，删除这笔流水不会改变任何钱包余额。"
				}
			}
			if ui.AlertDialog(c, &a.deleteEntryOpen, "删除这笔记录？", message, "取消", "删除") == 1 {
				a.confirmDeleteEntry(entry)
			}
		}
	}

	if a.deleteWalletOpen {
		wallet, ok := a.walletByID(a.deleteWalletID)
		if !ok {
			a.deleteWalletOpen = false
		} else {
			count := a.snap.WalletEntryCount[wallet.ID]
			message := fmt.Sprintf("有 %d 笔记录关联此钱包，删除后余额将不计入。相关流水会保留，但不再关联这只钱包。", count)
			if ui.AlertDialog(c, &a.deleteWalletOpen, fmt.Sprintf("删除钱包「%s」？", wallet.Name),
				message, "取消", "删除") == 1 {
				a.confirmDeleteWallet(wallet.ID)
			}
		}
	}
}

// monthLabelCN is the section heading month, e.g. "2026年11月".
func monthLabelCN(t time.Time) string {
	return fmt.Sprintf("%d年%d月", t.Year(), int(t.Month()))
}

// weekdayCN names a day of the week for a ledger row.
func weekdayCN(d time.Weekday) string {
	return [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[int(d)]
}
