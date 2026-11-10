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

var darkLook = look{
	background:   ui.Hex("#1C1916"),
	surface:      ui.Hex("#26211C"),
	surfaceAlt:   ui.Hex("#221E1A"),
	surfaceWarm:  ui.Hex("#302921"),
	border:       ui.Hex("#3D332A"),
	borderStrong: ui.Hex("#574A3D"),
	text:         ui.Hex("#F3E8DB"),
	muted:        ui.Hex("#B09B85"),
	accent:       ui.Hex("#8FBE9B"),
	accentSoft:   ui.Hex("#2B3B2E"),
	accentInk:    ui.Hex("#17251B"),
	coral:        ui.Hex("#E28A75"),
	coralSoft:    ui.Hex("#3A2925"),
	amber:        ui.Hex("#E0B261"),
	amberSoft:    ui.Hex("#3A3021"),
	mint:         ui.Hex("#94C6A3"),
	mintSoft:     ui.Hex("#29382D"),
	blue:         ui.Hex("#8AAECB"),
	blueSoft:     ui.Hex("#26313A"),
	plum:         ui.Hex("#B79AC0"),
	shadow:       ui.RGBA(0, 0, 0, 0.32),
	hairline:     ui.Hex("#3A3028"),
	stageTop:     ui.Hex("#273329"),
	stageBottom:  ui.Hex("#2A241E"),
}

func lookFor(c *ui.Context) look {
	if c.Theme().Dark {
		return darkLook
	}
	return lightLook
}

// applyTheme takes the system's theme and overlays WalletFeeder's warm
// palette, so native controls, focus rings, selection and text all follow
// the same visual language as the custom drawing.
func applyTheme(c *ui.Context) {
	t := *c.Theme()
	p := lookFor(c)
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
