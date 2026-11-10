package main

import (
	"math"

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

// drawLiquidBackdrop paints the aurora night the glass surfaces sample:
// a dark polar sky, stars, and curtains of mint, azure and violet light.
// There are no warm stop-light hues; what the glass blurs and bends comes
// from light rather than from flat decoration.
func (a *App) drawLiquidBackdrop(p *ui.Painter, r ui.Rect) {
	// Night sky, with a cool horizon glow instead of a flat black fill.
	p.FillGradient(r, ui.LinearGradient{
		From: ui.Hex("#02060D"), To: ui.Hex("#081A24"), Angle: 168,
	}, 0)
	for _, glow := range []struct {
		x, y, radius float32
		color        ui.Color
	}{
		{.28, .06, r.W * .62, ui.RGBA(35, 211, 164, .30)},
		{.82, .18, r.W * .52, ui.RGBA(47, 135, 255, .26)},
		{.18, .64, r.W * .50, ui.RGBA(116, 87, 255, .20)},
		{.62, .96, r.W * .60, ui.RGBA(20, 133, 143, .24)},
	} {
		p.FillRadialGradient(r, ui.RadialGradient{
			Center: ui.Point{X: glow.x, Y: glow.y},
			Radius: glow.radius,
			Stops: []ui.GradientStop{
				{Color: glow.color, At: 0},
				{Color: glow.color.Alpha(0), At: 1},
			},
		}, 0)
	}

	drawStars(p, r, .82)

	// The curtains are layered strokes: wide and faint outside, narrow and
	// bright at the core, so their edges dissolve into the sky.
	auroraCurtain(p, r, -.16, .72, .27, .040, .20, 1.3, r.H*.215, ui.Hex("#36E3B4"), 1.0)
	auroraCurtain(p, r, -.08, 1.16, .35, .050, 2.20, 1.05, r.H*.175, ui.Hex("#4D9FFF"), .82)
	auroraCurtain(p, r, .28, 1.18, .17, .030, -1.35, 1.7, r.H*.135, ui.Hex("#A98BFF"), .68)
	auroraCurtain(p, r, -.22, .88, .63, .055, 1.10, .9, r.H*.115, ui.Hex("#2ED9C2"), .46)

	// Fine vertical striations inside the big mint curtain: aurora rarely
	// glows as a perfect strip.
	for i := range 62 {
		t := float32(i) / 61
		x := r.X + r.W*(-.02+.68*t)
		base := r.Y + r.H*(.17+.028*float32(math.Sin(float64(t*7.2+.5))))
		length := r.H * (.035 + .085*float32(math.Abs(math.Sin(float64(i)*1.71))))
		strand := (&ui.Path{}).
			MoveTo(x, base).
			QuadTo(x+r.W*.008, base+length*.45, x+r.W*.013, base+length)
		color := ui.Hex("#6FF2D2")
		if i%4 == 0 {
			color = ui.Hex("#70B7FF")
		}
		p.StrokePath(strand, 1.35, color.Alpha(.055+.045*float32(math.Abs(math.Sin(float64(i)*.83)))))
	}

	// A cool lower haze grounds the frame without introducing another hue.
	p.FillGradient(r, ui.LinearGradient{
		From: ui.Transparent, To: ui.RGBA(8, 27, 38, .58), Angle: 180, Start: .55,
	}, 0)
}

// auroraCurtain draws one luminous band from x0 to x1, whose centre line
// wanders with a controlled sine. The layers produce a soft edge and a
// bright core, much like a real aurora's lower rim.
func auroraCurtain(p *ui.Painter, r ui.Rect, x0, x1, baseY, amplitude, phase, frequency, width float32, color ui.Color, strength float32) {
	path := &ui.Path{}
	const steps = 52
	for i := 0; i <= steps; i++ {
		t := float32(i) / steps
		x := r.X + r.W*(x0+(x1-x0)*t)
		wave := amplitude*float32(math.Sin(float64(t*frequency*2*math.Pi+phase))) +
			amplitude*.34*float32(math.Sin(float64(t*frequency*5.1*math.Pi+phase*1.9)))
		y := r.Y + r.H*(baseY+wave)
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	for _, layer := range []struct {
		width, alpha float32
	}{
		{width * 1.35, .030 * strength},
		{width * .94, .048 * strength},
		{width * .62, .075 * strength},
		{width * .34, .115 * strength},
		{width * .16, .18 * strength},
		{width * .05, .28 * strength},
	} {
		p.StrokePath(path, layer.width, color.Alpha(layer.alpha))
	}
}

// drawStars seeds a deterministic star field: the same sky every frame, no
// random flicker, while the glass can catch individual pinpricks of light.
func drawStars(p *ui.Painter, r ui.Rect, maxY float32) {
	seed := uint64(0x9E3779B97F4A7C15)
	next := func() float32 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return float32((seed>>40)&0xFFFFFF) / float32(1<<24)
	}
	for i := range 230 {
		nx, ny := next(), next()
		if ny > maxY {
			continue
		}
		x := r.X + nx*r.W
		y := r.Y + ny*r.H
		size := float32(.65) + next()*1.35
		a := float32(.20) + next()*.56
		p.Fill(ui.Rect{X: x, Y: y, W: size, H: size}, ui.RGBA(224, 248, 255, a), size/2)
		if i%29 == 0 {
			arm := size * 3.4
			p.Line(x-arm, y+size/2, x+arm, y+size/2, .7, ui.RGBA(207, 243, 255, a*.55))
			p.Line(x+size/2, y-arm, x+size/2, y+arm, .7, ui.RGBA(207, 243, 255, a*.55))
		}
	}
}
