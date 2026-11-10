package main

import "github.com/egoist/mygo/ui"

// look holds the few colors that are not part of ui.Theme but give
// WalletFeeder its own warm, paper-like character.
type look struct {
	background   ui.Color
	surface      ui.Color
	surfaceAlt   ui.Color
	surfaceWarm  ui.Color
	border       ui.Color
	borderStrong ui.Color
	text         ui.Color
	muted        ui.Color
	accent       ui.Color
	accentSoft   ui.Color
	accentInk    ui.Color
	coral        ui.Color
	coralSoft    ui.Color
	amber        ui.Color
	amberSoft    ui.Color
	mint         ui.Color
	mintSoft     ui.Color
	blue         ui.Color
	blueSoft     ui.Color
	plum         ui.Color
	shadow       ui.Color
	hairline     ui.Color
	stageTop     ui.Color
	stageBottom  ui.Color
}

var lightLook = look{
	background:   ui.Hex("#F6F0E7"),
	surface:      ui.Hex("#FFFCF7"),
	surfaceAlt:   ui.Hex("#FAF4EC"),
	surfaceWarm:  ui.Hex("#F2E8DA"),
	border:       ui.Hex("#E8DDD0"),
	borderStrong: ui.Hex("#D6C6B4"),
	text:         ui.Hex("#3F3328"),
	muted:        ui.Hex("#8E7E6F"),
	accent:       ui.Hex("#4E7C5D"),
	accentSoft:   ui.Hex("#E5F0E6"),
	accentInk:    ui.Hex("#2F5840"),
	coral:        ui.Hex("#C77059"),
	coralSoft:    ui.Hex("#F7E4DD"),
	amber:        ui.Hex("#B9853A"),
	amberSoft:    ui.Hex("#F8EDD6"),
	mint:         ui.Hex("#679B7A"),
	mintSoft:     ui.Hex("#E7F1E8"),
	blue:         ui.Hex("#5C83A6"),
	blueSoft:     ui.Hex("#E6EEF5"),
	plum:         ui.Hex("#8A6B95"),
	shadow:       ui.RGBA(70, 50, 30, 0.10),
	hairline:     ui.Hex("#E8DDD0"),
	stageTop:     ui.Hex("#EAF3E9"),
	stageBottom:  ui.Hex("#F8F1E6"),
}

// darkLook is the aurora palette: a cold night sky with mint aurora glass,
// icy type and enough luminance for small labels. Glass mode always uses
// it, even when the desktop is in light mode; the paper look still follows
// the desktop.
var darkLook = look{
	background:   ui.Hex("#06131C"),
	surface:      ui.Hex("#0C1B24"),
	surfaceAlt:   ui.Hex("#09161E"),
	surfaceWarm:  ui.Hex("#132630"),
	border:       ui.Hex("#223844"),
	borderStrong: ui.Hex("#355463"),
	text:         ui.Hex("#EDF7F5"),
	muted:        ui.Hex("#8FAEB4"),
	accent:       ui.Hex("#74E3C3"),
	accentSoft:   ui.Hex("#0F2B27"),
	accentInk:    ui.Hex("#06211D"),
	coral:        ui.Hex("#F0899D"),
	coralSoft:    ui.Hex("#351C26"),
	amber:        ui.Hex("#E7C27B"),
	amberSoft:    ui.Hex("#322B1C"),
	mint:         ui.Hex("#74E3C3"),
	mintSoft:     ui.Hex("#102B28"),
	blue:         ui.Hex("#86C8FF"),
	blueSoft:     ui.Hex("#10293A"),
	plum:         ui.Hex("#B8A4FF"),
	shadow:       ui.RGBA(0, 0, 0, 0.34),
	hairline:     ui.Hex("#1A303A"),
	stageTop:     ui.Hex("#0C2228"),
	stageBottom:  ui.Hex("#07161D"),
}

func lookFor(c *ui.Context) look {
	if c.Theme().Dark {
		return darkLook
	}
	return lightLook
}

// applyTheme overlays WalletFeeder's palette, so native controls, focus
// rings, selection and text all follow the same visual language as the
// custom drawing. The aurora glass look is always dark for contrast; the
// paper look follows the desktop's appearance.
func (a *App) applyTheme(c *ui.Context) {
	t := *c.Theme()
	dark := t.Dark || a.useGlass
	p := lightLook
	if dark {
		p = darkLook
	}
	t.Dark = dark
	t.Background = p.background
	t.Surface = p.surface
	t.SurfaceHover = p.surfaceWarm
	t.SurfacePressed = p.surfaceWarm
	t.Border = p.border
	t.Text = p.text
	t.TextMuted = p.muted
	t.Accent = p.accent
	t.AccentHover = p.accent.Mix(p.text, 0.08)
	t.AccentPressed = p.accent.Mix(p.text, 0.16)
	t.AccentText = ui.Hex("#FFFDF8")
	if t.Dark {
		t.AccentText = ui.Hex("#14251A")
	}
	t.InactiveAccent = p.accent
	t.Danger = p.coral
	t.Warning = p.amber
	t.Success = p.mint
	t.Selection = p.accent.Alpha(0.24)
	t.Focus = p.accent
	t.Inverse = p.text
	t.InverseText = p.background
	t.Scrollbar = p.borderStrong
	t.Radius = 12
	t.Spacing = 4
	t.FocusRingWidth = 2
	t.FocusRingOffset = 2
	c.SetTheme(&t)
}

// alpha is a tiny shorthand for translucent custom colors.
func alpha(c ui.Color, a float32) ui.Color { return c.Alpha(a) }
