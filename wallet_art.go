package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// stageColors are the colors of the wallet's little stage and body during
// one mood. They are separate from ui.Theme because the character is a
// brand illustration, not a system control.
type stageColors struct {
	top, bottom ui.Color
	body        ui.Color
	bodyDark    ui.Color
	lip         ui.Color
	face        ui.Color
	blush       ui.Color
	outline     ui.Color
	doodle      ui.Color
}

func stagePalette(m Mood, negative bool, l look) stageColors {
	if negative {
		if l.text == lightLook.text {
			return stageColors{
				top: ui.Hex("#EEF2F4"), bottom: ui.Hex("#F8F3EB"),
				body: ui.Hex("#BDC9D1"), bodyDark: ui.Hex("#9DACB7"),
				lip: ui.Hex("#E4EBEF"), face: ui.Hex("#34424D"),
				blush: ui.Hex("#D79C91"), outline: ui.Hex("#7E909D"),
				doodle: ui.Hex("#CFDDE4"),
			}
		}
		return stageColors{
			top: ui.Hex("#2A3136"), bottom: ui.Hex("#241F1B"),
			body: ui.Hex("#AAB9C3"), bodyDark: ui.Hex("#8394A0"),
			lip: ui.Hex("#D8E1E6"), face: ui.Hex("#29343D"),
			blush: ui.Hex("#C8877B"), outline: ui.Hex("#657A88"),
			doodle: ui.Hex("#3D4A52"),
		}
	}
	switch m {
	case MoodPlump:
		if l.text == lightLook.text {
			return stageColors{
				top: ui.Hex("#EAF4E9"), bottom: ui.Hex("#F8F2E7"),
				body: ui.Hex("#96C9A4"), bodyDark: ui.Hex("#6FAB7F"),
				lip: ui.Hex("#D9ECD9"), face: ui.Hex("#274A35"),
				blush: ui.Hex("#EEA095"), outline: ui.Hex("#5C9070"),
				doodle: ui.Hex("#C8E2CC"),
			}
		}
		return stageColors{
			top: ui.Hex("#28372C"), bottom: ui.Hex("#242019"),
			body: ui.Hex("#86B994"), bodyDark: ui.Hex("#63976F"),
			lip: ui.Hex("#C9E2CC"), face: ui.Hex("#1B3424"),
			blush: ui.Hex("#D28B7E"), outline: ui.Hex("#4E785A"),
			doodle: ui.Hex("#3F5546"),
		}
	case MoodOkay:
		if l.text == lightLook.text {
			return stageColors{
				top: ui.Hex("#FAF0DC"), bottom: ui.Hex("#FBF6ED"),
				body: ui.Hex("#F0C87E"), bodyDark: ui.Hex("#D8A75B"),
				lip: ui.Hex("#FBE8C4"), face: ui.Hex("#5A4122"),
				blush: ui.Hex("#EA9C88"), outline: ui.Hex("#B78645"),
				doodle: ui.Hex("#EED8AF"),
			}
		}
		return stageColors{
			top: ui.Hex("#39301F"), bottom: ui.Hex("#252019"),
			body: ui.Hex("#D8B26A"), bodyDark: ui.Hex("#B68D47"),
			lip: ui.Hex("#F3E1B9"), face: ui.Hex("#392A14"),
			blush: ui.Hex("#C98775"), outline: ui.Hex("#8E6D38"),
			doodle: ui.Hex("#54462F"),
		}
	default:
		if l.text == lightLook.text {
			return stageColors{
				top: ui.Hex("#F7E9E4"), bottom: ui.Hex("#FBF6EE"),
				body: ui.Hex("#E3ADA3"), bodyDark: ui.Hex("#C98B80"),
				lip: ui.Hex("#F8DDD6"), face: ui.Hex("#5D3933"),
				blush: ui.Hex("#D98579"), outline: ui.Hex("#B07368"),
				doodle: ui.Hex("#EAC7C0"),
			}
		}
		return stageColors{
			top: ui.Hex("#3B2A26"), bottom: ui.Hex("#26201C"),
			body: ui.Hex("#C9968B"), bodyDark: ui.Hex("#A8766C"),
			lip: ui.Hex("#EDD2CB"), face: ui.Hex("#3A221E"),
			blush: ui.Hex("#B76F64"), outline: ui.Hex("#8B5A51"),
			doodle: ui.Hex("#5A3E39"),
		}
	}
}

// drawBrandMark is the tiny purse in the title bar. It is drawn rather than
// loaded so the header starts with the same primitives as the rest.
func drawBrandMark(p *ui.Painter, r ui.Rect, l look) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	w, h := r.W*0.56, r.H*0.46
	body := ui.Rect{X: cx - w/2, Y: cy - h/2 + r.H*0.06, W: w, H: h}
	p.Shadow(body, h*0.35, 0, 3, 6, 0, alpha(l.text, 0.10))
	p.Fill(body, l.accent, h*0.34)
	lid := ui.Rect{X: body.X, Y: body.Y, W: body.W, H: body.H * 0.36}
	p.Fill(lid, l.accentInk, h*0.34)
	p.Line(body.X+body.W*0.2, body.Y+body.H*0.4, body.X+body.W*0.8, body.Y+body.H*0.4, 1, alpha(l.surface, 0.5))
	// face
	eyeY := body.Y + body.H*0.67
	eyeGap := body.W * 0.17
	p.Fill(ui.Rect{X: cx - eyeGap - 1.3, Y: eyeY - 1.3, W: 2.6, H: 2.6}, l.surface, 2)
	p.Fill(ui.Rect{X: cx + eyeGap - 1.3, Y: eyeY - 1.3, W: 2.6, H: 2.6}, l.surface, 2)
	mouth := (&ui.Path{}).MoveTo(cx-3.5, eyeY+3).
		QuadTo(cx, eyeY+6.2, cx+3.5, eyeY+3)
	p.StrokePath(mouth, 1.3, alpha(l.surface, 0.9))
	// clasp
	p.Fill(ui.Rect{X: cx - 2.5, Y: body.Y + body.H*0.36 - 2.5, W: 5, H: 5}, l.surfaceWarm, 2.5)
}

// drawWalletCreature paints the wallet at the center of r. fullness is
// 0.35..0.9 for a slim..plump body. reaction is true for the short window
// after a feed.
func drawWalletCreature(p *ui.Painter, r ui.Rect, m Mood, negative bool, fullness float32, reaction bool, reactionKind Kind, l look) {
	pal := stagePalette(m, negative, l)
	cx, cy := r.X+r.W/2, r.Y+r.H/2

	// Decorative stage: a big soft halo, little dots, and a sparkle.
	haloR := minF(r.W, r.H) * 0.42
	p.Fill(ui.Rect{
		X: cx - haloR, Y: cy - haloR*0.92,
		W: haloR * 2, H: haloR * 1.84,
	}, alpha(pal.doodle, 0.45), haloR)
	drawSparkle(p, cx+r.W*0.31, cy-r.H*0.30, 6, alpha(pal.outline, 0.6))
	drawSparkle(p, cx-r.W*0.33, cy-r.H*0.15, 4, alpha(pal.outline, 0.38))
	drawDot(p, cx-r.W*0.28, cy+r.H*0.25, 2.5, alpha(pal.outline, 0.3))
	drawDot(p, cx+r.W*0.34, cy+r.H*0.20, 2, alpha(pal.outline, 0.28))

	// Body proportions follow the ratio of balance to budget.
	if fullness < 0.38 {
		fullness = 0.38
	}
	bodyW := minF(r.W*0.60, 250) * (0.78 + 0.34*fullness)
	bodyH := minF(r.H*0.68, 175) * (0.78 + 0.30*fullness)
	if negative {
		bodyW *= 0.92
		bodyH *= 0.90
	}

	bob := float32(0)
	chew := float32(0)
	if reaction {
		t := float32(p.Now().UnixMilli()%1000) / 1000
		bob = -5 - 3*float32(math.Sin(float64(t)*math.Pi*2))
		chew = 2.5 * float32(math.Sin(float64(t)*math.Pi*2))
		p.AnimationFrame()
	}

	body := ui.Rect{
		X: cx - bodyW/2 + chew,
		Y: cy - bodyH/2 + bob,
		W: bodyW,
		H: bodyH,
	}
	radius := minF(bodyW, bodyH) * 0.33

	// Ground shadow.
	p.Fill(ui.Rect{
		X: body.X + body.W*0.12,
		Y: body.Y + body.H*0.94,
		W: body.W * 0.76,
		H: 13,
	}, alpha(l.text, 0.10), 7)

	// The purse body, with a slightly darker flap and a stitched seam.
	p.Shadow(body, radius, 0, 10, 24, 0, alpha(l.text, 0.14))
	p.FillGradient(body, ui.LinearGradient{
		From: pal.body, To: pal.bodyDark, Angle: 115,
	}, radius)
	lid := ui.Rect{X: body.X, Y: body.Y, W: body.W, H: body.H * 0.33}
	p.Clip(body, radius, func() {
		p.Fill(lid, pal.lip, radius)
		seamY := body.Y + body.H*0.33
		stitch := (&ui.Path{}).MoveTo(body.X+body.W*0.12, seamY+5).
			LineTo(body.X+body.W*0.88, seamY+5)
		p.StrokePath(stitch.Dash(0, 5, 3).Cap(ui.ButtCap), 1.1, alpha(pal.outline, 0.5))
		// a soft highlight on the belly
		p.Fill(ui.Rect{
			X: body.X + body.W*0.20, Y: body.Y + body.H*0.48,
			W: body.W * 0.22, H: body.H * 0.14,
		}, alpha(ui.Hex("#FFFFFF"), 0.14), body.H*0.07)
	})
	p.Stroke(body, alpha(pal.outline, 0.65), radius, 1.2)

	// The clasp sits on the seam at the right, like a little brass button.
	clasp := ui.Rect{
		X: body.X + body.W*0.82, Y: body.Y + body.H*0.33 - 5.5,
		W: 11, H: 11,
	}
	p.Fill(clasp, ui.Hex("#E9C66E"), 5.5)
	p.Stroke(clasp, alpha(ui.Hex("#7B5D28"), 0.55), 5.5, 1)
	p.Fill(ui.Rect{X: clasp.X + 3.7, Y: clasp.Y + 3.7, W: 3.6, H: 3.6}, ui.Hex("#FFF1B9"), 1.8)

	faceY := body.Y + body.H*0.64
	eyeGap := body.W * 0.145
	eyeY := faceY - 5

	if negative || m == MoodSlim {
		// Worried eyes: tiny dots with gently sloping brows.
		for _, side := range []float32{-1, 1} {
			ex := cx + side*eyeGap
			p.Fill(ui.Rect{X: ex - 2.2, Y: eyeY - 2.2, W: 4.4, H: 4.4}, pal.face, 2.2)
			brow := (&ui.Path{}).MoveTo(ex+side*6, eyeY-9).
				LineTo(ex-side*3, eyeY-7)
			p.StrokePath(brow, 1.5, alpha(pal.face, 0.75))
		}
	} else if reaction || m == MoodPlump {
		// Happy closed eyes, like two little hills.
		for _, side := range []float32{-1, 1} {
			ex := cx + side*eyeGap
			eye := (&ui.Path{}).MoveTo(ex-7, eyeY).
				QuadTo(ex, eyeY-7, ex+7, eyeY)
			p.StrokePath(eye, 2.4, pal.face)
			if reaction {
				// a spark in each eye while chewing
				drawDot(p, ex+2, eyeY-3, 1.4, alpha(ui.Hex("#FFFDF6"), 0.9))
			}
		}
	} else {
		for _, side := range []float32{-1, 1} {
			ex := cx + side*eyeGap
			p.Fill(ui.Rect{X: ex - 2.8, Y: eyeY - 3.2, W: 5.6, H: 6.4}, pal.face, 3)
			drawDot(p, ex-0.8, eyeY-1, 1.1, alpha(ui.Hex("#FFFFFF"), 0.8))
		}
	}

	// Blush.
	for _, side := range []float32{-1, 1} {
		ex := cx + side*body.W*0.25
		p.Fill(ui.Rect{X: ex - 10, Y: faceY + 3, W: 20, H: 8}, alpha(pal.blush, 0.46), 4)
	}

	// Mouth.
	mouthY := faceY + 13
	switch {
	case reaction:
		if reactionKind == KindIncome {
			p.Fill(ui.Rect{X: cx - 7, Y: mouthY - 4, W: 14, H: 10}, pal.face, 5)
			p.Fill(ui.Rect{X: cx - 3, Y: mouthY + 1, W: 6, H: 3}, alpha(ui.Hex("#F2A7A0"), 0.9), 2)
		} else {
			p.Fill(ui.Rect{X: cx - 6, Y: mouthY - 3, W: 12, H: 9}, pal.face, 4.5)
			p.Fill(ui.Rect{X: cx - 2.5, Y: mouthY + 1.5, W: 5, H: 2.5}, alpha(ui.Hex("#F2A7A0"), 0.9), 1.5)
		}
	case negative:
		frown := (&ui.Path{}).MoveTo(cx-6, mouthY+4).
			QuadTo(cx, mouthY-3, cx+6, mouthY+4)
		p.StrokePath(frown, 1.9, pal.face)
	case m == MoodSlim:
		flat := (&ui.Path{}).MoveTo(cx-5, mouthY+1).
			QuadTo(cx, mouthY-1.5, cx+5, mouthY+1)
		p.StrokePath(flat, 1.8, pal.face)
	default:
		smile := (&ui.Path{}).MoveTo(cx-7, mouthY-1).
			QuadTo(cx, mouthY+7, cx+7, mouthY-1)
		p.StrokePath(smile, 1.9, pal.face)
	}

	if negative {
		// A single sweat drop, drawn as a little path.
		drop := (&ui.Path{}).
			MoveTo(body.X+body.W*0.92, body.Y+body.H*0.52).
			QuadTo(body.X+body.W*0.96, body.Y+body.H*0.58, body.X+body.W*0.92, body.Y+body.H*0.62).
			QuadTo(body.X+body.W*0.88, body.Y+body.H*0.58, body.X+body.W*0.92, body.Y+body.H*0.52)
		p.FillPath(drop, alpha(ui.Hex("#76A9C8"), 0.8))
	}

	if reaction {
		// A little coin (or a receipt for income) floats up from the mouth.
		coinX := cx + body.W*0.30
		coinY := body.Y - 8 - 8*float32(math.Sin(float64(p.Now().UnixMilli()%600)/600*math.Pi))
		coin := ui.Rect{X: coinX - 10, Y: coinY, W: 20, H: 20}
		p.Shadow(coin, 10, 0, 4, 8, 0, alpha(l.text, 0.16))
		p.Fill(coin, ui.Hex("#F0CC78"), 10)
		p.Stroke(coin, alpha(ui.Hex("#8F6A22"), 0.65), 10, 1.2)
		if reactionKind == KindIncome {
			p.Line(coinX-3.5, coinY+5, coinX-3.5, coinY+14, 1.6, ui.Hex("#8F6A22"))
			p.Line(coinX+3.5, coinY+5, coinX+3.5, coinY+14, 1.6, ui.Hex("#8F6A22"))
		} else {
			p.Text(coinX-3.5, coinY+14.5, "¥", 9, ui.Hex("#8F6A22"))
		}
		for i, dx := range []float32{-18, -9, 16} {
			dy := []float32{16, 24, 10}[i]
			p.Fill(ui.Rect{X: cx + dx, Y: body.Y + body.H*0.5 + dy, W: 3, H: 3},
				alpha(pal.outline, 0.55), 1.5)
		}
	}
}

func drawSparkle(p *ui.Painter, x, y, size float32, c ui.Color) {
	path := (&ui.Path{}).
		MoveTo(x, y-size).LineTo(x+size*0.25, y-size*0.25).
		LineTo(x+size, y).LineTo(x+size*0.25, y+size*0.25).
		LineTo(x, y+size).LineTo(x-size*0.25, y+size*0.25).
		LineTo(x-size, y).LineTo(x-size*0.25, y-size*0.25).Close()
	p.FillPath(path, c)
}

func drawDot(p *ui.Painter, x, y, r float32, c ui.Color) {
	p.Fill(ui.Rect{X: x - r, Y: y - r, W: 2 * r, H: 2 * r}, c, r)
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
