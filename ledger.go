package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

func (a *App) ledgerPage(c *ui.Context) {
	p := lookFor(c)
	ui.Column(c).Fill().Padding(14, 28, 16).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "流水").FontSize(22).FontWeight(700).TextColor(p.text)
				ui.Text(c, "钱来钱去，每一笔都算数。").FontSize(11).TextColor(p.muted)
			})
			ui.Spacer(c)
			a.monthSwitcher(c)
		})

		a.ledgerSummary(c)

		switch {
		case a.ledgerLoading:
			ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
				ui.Text(c, "正在翻这个月的账本…").FontSize(12).TextColor(p.muted)
			})
		case len(a.snap.Ledger) == 0:
			ui.Column(c).Grow(1).Center().Children(func() {
				empty := ui.Column(c).Width(420).Padding(24).Gap(8).Center()
				empty = a.panelAlt(c, empty, 18)
				empty.Children(func() {
					ui.Text(c, "这个月还没有流水").FontSize(15).FontWeight(600).TextColor(p.text)
					ui.Text(c, "在首页喂一口，或者换个月份看看。").FontSize(11).TextColor(p.muted).
						TextAlign(ui.Center)
					if ui.Button(c, "回到首页").Label("回到首页").Clicked() {
						a.setPage(pageHome)
					}
				})
			})
		default:
			a.ledgerListCard(c)
		}
	})
}

func (a *App) monthSwitcher(c *ui.Context) {
	p := lookFor(c)
	monthName := monthLabelCN(a.ledgerMonth)
	ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
		prev := ui.ButtonBase(c).Size(32, 32).Radius(999).Label("上一月").Tooltip("上一月").Disabled(a.busy)
		if a.useGlass {
			prev.Material(glass.Glass{Interactive: true}).Border(1, ui.RGBA(255, 255, 255, .3))
		} else {
			prev.Background(p.surface).Border(1, p.border)
		}
		prev.Children(func() { ui.Text(c, "‹").FontSize(18).TextColor(p.text) })
		if prev.Clicked() {
			a.shiftLedgerMonth(-1)
		}

		monthBox := ui.Box(c).Height(32).MinWidth(120).Padding(0, 12).Radius(999)
		if a.useGlass {
			monthBox.Material(glass.Glass{Clarity: .45}).Border(1, ui.RGBA(255, 255, 255, .26))
		} else {
			monthBox.Background(p.surface).Border(1, p.border)
		}
		monthBox.Children(func() {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Text(c, monthName).FontSize(12).FontWeight(600).TextColor(p.text)
			})
		})
		next := ui.ButtonBase(c).Size(32, 32).Radius(999).Label("下一月").Tooltip("下一月").Disabled(a.busy)
		if a.useGlass {
			next.Material(glass.Glass{Interactive: true}).Border(1, ui.RGBA(255, 255, 255, .3))
		} else {
			next.Background(p.surface).Border(1, p.border)
		}
		next.Children(func() { ui.Text(c, "›").FontSize(18).TextColor(p.text) })
		if next.Clicked() {
			a.shiftLedgerMonth(1)
		}
		if !monthStart(a.ledgerMonth).Equal(a.currentMonth()) {
			back := ui.Button(c, "回到本月").Label("回到本月").Disabled(a.busy)
			if a.useGlass {
				back.Material(glass.Glass{Clarity: .4, Interactive: true})
			}
			if back.Clicked() {
				a.goToCurrentMonth()
			}
		}
	})
}

func (a *App) ledgerSummary(c *ui.Context) {
	p := lookFor(c)
	title := "本月合计"
	if !monthStart(a.ledgerMonth).Equal(a.currentMonth()) {
		title = monthLabelCN(a.ledgerMonth) + "合计"
	}
	net := a.snap.LedgerStats.IncomeCents - a.snap.LedgerStats.ExpenseCents
	bar := a.panel(c, ui.Row(c).Height(56).Padding(8, 16).AlignItems(ui.Center), 15)
	bar.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Width(140).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(p.accent)
			ui.Text(c, title).FontSize(12).FontWeight(700).TextColor(p.text)
		})
		summaryCell(c, "支出", FormatCents(a.snap.LedgerStats.ExpenseCents), p.coral)
		ui.Box(c).Width(1).Height(22).Background(p.hairline)
		summaryCell(c, "收入", FormatCents(a.snap.LedgerStats.IncomeCents), p.mint)
		ui.Box(c).Width(1).Height(22).Background(p.hairline)
		summaryCell(c, "结余", FormatDelta(net), netColor(net, p))
		ui.Spacer(c)
		ui.Text(c, strings.TrimSpace(strings.Join([]string{
			fmt.Sprintf("%d 笔记录", len(a.snap.Ledger)),
			"只记支出与收入",
		}, " · "))).FontSize(9.5).TextColor(p.muted)
	})
}

func summaryCell(c *ui.Context, label, value string, accent ui.Color) {
	ui.Row(c).Gap(5).AlignItems(ui.Baseline).Padding(0, 10).Children(func() {
		ui.Text(c, label).FontSize(10).TextColor(lookFor(c).muted)
		ui.Text(c, value).FontSize(13).FontWeight(650).TextColor(accent)
	})
}

func netColor(net int64, p look) ui.Color {
	if net < 0 {
		return p.coral
	}
	return p.accentInk
}

func (a *App) ledgerListCard(c *ui.Context) {
	entries := a.snap.Ledger
	card := a.panel(c, ui.Column(c).Grow(1), 16)
	card.Clip().Padding(4, 0).Children(func() {
		a.ledgerList.Key = func(i int) any { return entries[i].ID }
		ui.List(c, &a.ledgerList, len(entries), func(i int) {
			a.ledgerRow(c, entries[i])
		}).Grow(1)
	})
}

func (a *App) ledgerRow(c *ui.Context, e Entry) {
	p := lookFor(c)
	income := e.Kind == KindIncome
	amountText := "−" + FormatCents(e.AmountCents)
	amountColor := p.text
	if income {
		amountText = "+" + FormatCents(e.AmountCents)
		amountColor = p.mint
	}
	when := e.OccurredAt.Local()
	note := strings.TrimSpace(e.Note)
	metaParts := []string{e.WalletName}
	if note != "" {
		metaParts = append(metaParts, note)
	}
	meta := strings.Join(metaParts, " · ")

	row := ui.Row(c).Height(58).Padding(4, 12).Gap(10).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(p.hairline)
	row.Children(func() {
		ui.Column(c).Width(52).Shrink(0).Gap(1).Children(func() {
			ui.Textf(c, "%02d/%02d", int(when.Month()), when.Day()).
				FontSize(12).FontWeight(600).TextColor(p.text)
			ui.Text(c, weekdayCN(when.Weekday())+" "+when.Format("15:04")).
				FontSize(8.5).TextColor(p.muted).SingleLine()
		})
		ui.Box(c).Size(32, 32).Radius(10).Background(p.surfaceAlt).Children(func() {
			ui.Text(c, categoryEmoji(e.Category)).FontSize(15).Center()
		})
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Text(c, e.Category).FontSize(11.5).FontWeight(600).TextColor(p.text)
				if e.Adjustment {
					ui.Text(c, "校准").FontSize(8.5).FontWeight(600).TextColor(p.muted).
						Padding(2, 5).Radius(5).Background(p.surfaceWarm)
				}
			})
			ui.Text(c, meta).FontSize(9.5).TextColor(p.muted).SingleLine()
		})
		ui.Column(c).Width(120).Shrink(0).AlignItems(ui.End).Gap(1).Children(func() {
			ui.Text(c, amountText).FontSize(13).FontWeight(650).TextColor(amountColor).SingleLine()
			ui.Text(c, labelForKind(e.Kind)).FontSize(8.5).TextColor(p.muted)
		})
		del := ui.ButtonBase(c).Size(26, 26).Radius(7).Label(deleteEntryLabel(e)).
			Tooltip("删除这笔记录").Disabled(a.busy)
		if del.Hovered() {
			if a.useGlass {
				del.Background(p.coral.Alpha(.20))
			} else {
				del.Background(p.coralSoft)
			}
		}
		del.Children(func() {
			ui.Text(c, "✕").FontSize(11).TextColor(p.muted)
		})
		if del.Clicked() {
			a.askDeleteEntry(e.ID)
		}
	})
}

func labelForKind(k Kind) string {
	if k == KindIncome {
		return "收入"
	}
	return "支出"
}

func deleteEntryLabel(e Entry) string {
	return fmt.Sprintf("删除流水：%s %s %s", e.OccurredAt.Format("01-02"), e.Category, FormatCents(e.AmountCents))
}
