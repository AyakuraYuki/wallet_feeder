package main

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// useGlass starts on: WalletFeeder is meant to be looked at as much as
// used, and this is the best demonstration of MyGo's Liquid Glass. The
// header switch keeps the original paper look one click away.
func (a *App) glassMode() bool { return a.useGlass }

// panel gives a large surface the Liquid Glass material, or the original
// paper card when the appearance is switched. Glass draws its own rim and
// carries no shadow, so neither is added here.
func (a *App) panel(c *ui.Context, e ui.Element, radius float32) ui.Element {
	if a.useGlass {
		p := lookFor(c)
		rim := ui.RGBA(255, 255, 255, .38)
		if p.background == darkLook.background {
			rim = ui.RGBA(255, 255, 255, .20)
		}
		tint := p.surface.Alpha(.28)
		if p.background == darkLook.background {
			tint = ui.RGBA(5, 16, 24, .14)
		}
		return e.Radius(radius).Material(glass.Glass{Style: glass.Clear, Tint: tint, Clarity: .35}).Border(1, rim)
	}
	p := lookFor(c)
	return e.Radius(radius).Background(p.surface).Border(1, p.border)
}

// panelAlt is a quieter glass surface: small chips and inner cards where
// regular glass would frost too much. Paper mode uses the alternate
// surface color.
func (a *App) panelAlt(c *ui.Context, e ui.Element, radius float32) ui.Element {
	if a.useGlass {
		p := lookFor(c)
		rim := ui.RGBA(255, 255, 255, .26)
		if p.background == darkLook.background {
			rim = ui.RGBA(255, 255, 255, .15)
		}
		tint := p.surface.Alpha(.20)
		if p.background == darkLook.background {
			tint = ui.RGBA(5, 16, 24, .11)
		}
		return e.Radius(radius).Material(glass.Glass{Style: glass.Clear, Tint: tint, Clarity: .5}).Border(1, rim)
	}
	p := lookFor(c)
	return e.Radius(radius).Background(p.surfaceAlt).Border(1, p.border)
}

// tintedPanel is the "prominent" Liquid Glass surface: selected pills,
// the stage tint and primary actions. An opaque tint keeps the accent's
// contrast, as AppKit's prominent glass does.
func (a *App) tintedPanel(c *ui.Context, e ui.Element, radius float32, tint ui.Color, interactive bool) ui.Element {
	if a.useGlass {
		return e.Radius(radius).Material(glass.Glass{Tint: tint, Interactive: interactive})
	}
	return e.Radius(radius).Background(tint)
}

func (a *App) glassButton(c *ui.Context, label, accessible string, tint ui.Color, textColor ui.Color) ui.Element {
	b := ui.ButtonBase(c).Padding(6, 14).Radius(999).Height(34).Label(accessible)
	if a.useGlass {
		b.Material(glass.Glass{Tint: tint, Interactive: true})
	} else {
		b.Background(tint)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(textColor)
	})
	return b
}

// smallGlassButton is a compact icon/text button that still accepts the
// glass press animation.
func (a *App) smallGlassButton(c *ui.Context, label, accessible string) ui.Element {
	b := ui.ButtonBase(c).Padding(4, 8).Radius(8).Label(accessible).Tooltip(accessible)
	if a.useGlass {
		b.Material(glass.Glass{Clarity: .5, Interactive: true})
	} else {
		p := lookFor(c)
		b.Background(p.surfaceAlt).Border(1, p.border)
	}
	b.Children(func() {
		p := lookFor(c)
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(p.text)
	})
	return b
}

// fieldFrame wraps a control in glass in the new look and keeps the
// original crisp input in the paper look.
func (a *App) fieldFrame(c *ui.Context, e ui.Element, radius float32) ui.Element {
	if a.useGlass {
		return e.Radius(radius).Material(glass.Glass{Clarity: .28})
	}
	return e.Radius(radius)
}

// transparentInput removes the text input's own opaque skin while it sits
// inside a glass field.
func (a *App) transparentInput(e ui.Element) ui.Element {
	if !a.useGlass {
		return e
	}
	return e.Background(ui.Transparent).BorderWidth(0)
}

// drawLiquidBackdrop 按窗口的实际设备像素大小绘制并缓存独立位图。
// 大幅 StrokePath 蒙版与普通文字共用 MyGo 图集：整幅极光在 Retina 下会耗尽它，
// 让所有单色文字无法绘制。改用独立纹理后不再挤占文字图集，静止时也无需重复光栅化。
func (a *App) drawLiquidBackdrop(p *ui.Painter, r ui.Rect) {
	scale := p.Scale()
	if scale <= 0 {
		scale = 1
	}
	w, h := deviceLength(r.W, scale), deviceLength(r.H, scale)
	if w <= 0 || h <= 0 {
		return
	}
	if a.backdrop == nil || a.backdrop.width != w || a.backdrop.height != h || a.backdrop.scale != scale {
		a.backdrop = newAuroraBackdrop(w, h, scale)
	}
	p.ImageRegion(a.backdrop.bitmap, ui.Rect{}, r, ui.ImageOptions{})
}
