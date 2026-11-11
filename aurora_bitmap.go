package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/vector"
)

// auroraBackdrop 保存一张独立于字形图集的背景位图。
// 窗口静止时复用它，只有逻辑尺寸或屏幕倍率变化时才重新生成。
type auroraBackdrop struct {
	bitmap *ui.Bitmap
	width  int
	height int
	scale  float32
}

func deviceLength(v, scale float32) int {
	return int(math.Round(float64(v * scale)))
}

func newAuroraBackdrop(w, h int, scale float32) *auroraBackdrop {
	r := ui.Rect{W: float32(w) / scale, H: float32(h) / scale}
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// 与 Painter 的线性渐变一致：中心投影像素越接近 To 就越蓝。
	paintLinearGradient(img, r, scale, ui.Hex("#02060D"), ui.Hex("#081A24"), 168, 0)
	for _, glow := range []struct {
		x, y, radius float32
		color        ui.Color
	}{
		{.28, .06, r.W * .62, ui.RGBA(35, 211, 164, .30)},
		{.82, .18, r.W * .52, ui.RGBA(47, 135, 255, .26)},
		{.18, .64, r.W * .50, ui.RGBA(116, 87, 255, .20)},
		{.62, .96, r.W * .60, ui.RGBA(20, 133, 143, .24)},
	} {
		paintRadialGlow(img, glow.x*r.W*scale, glow.y*r.H*scale, glow.radius*scale, glow.color)
	}

	drawStars(img, r, scale, .82)

	// 光带仍然在设备精度下光栅化，但只进入这张独立位图，不再占用文字图集。
	auroraCurtain(img, r, scale, -.16, .72, .27, .040, .20, 1.3, r.H*.215, ui.Hex("#36E3B4"), 1.0)
	auroraCurtain(img, r, scale, -.08, 1.16, .35, .050, 2.20, 1.05, r.H*.175, ui.Hex("#4D9FFF"), .82)
	auroraCurtain(img, r, scale, .28, 1.18, .17, .030, -1.35, 1.7, r.H*.135, ui.Hex("#A98BFF"), .68)
	auroraCurtain(img, r, scale, -.22, .88, .63, .055, 1.10, .9, r.H*.115, ui.Hex("#2ED9C2"), .46)

	for i := range 62 {
		t := float32(i) / 61
		x := r.W * (-.02 + .68*t) * scale
		base := r.H * (.17 + .028*float32(math.Sin(float64(t*7.2+.5)))) * scale
		length := r.H * (.035 + .085*float32(math.Abs(math.Sin(float64(i)*1.71)))) * scale
		pts := make([]bitmapPoint, 0, 13)
		for j := range 13 {
			u := float32(j) / 12
			v := 1 - u
			pts = append(pts, bitmapPoint{
				x: v*v*x + 2*v*u*(x+r.W*.008*scale) + u*u*(x+r.W*.013*scale),
				y: v*v*base + 2*v*u*(base+length*.45) + u*u*(base+length),
			})
		}
		color := ui.Hex("#6FF2D2")
		if i%4 == 0 {
			color = ui.Hex("#70B7FF")
		}
		paintStroke(img, pts, 1.35*scale, color.Alpha(.055+.045*float32(math.Abs(math.Sin(float64(i)*.83)))))
	}

	paintLinearGradient(img, r, scale, ui.Transparent, ui.RGBA(8, 27, 38, .58), 180, .55)
	return &auroraBackdrop{bitmap: ui.NewBitmap(img), width: w, height: h, scale: scale}
}

// paintLinearGradient 用 sRGB 预乘颜色合成渐变，保持与 Painter 相同的色彩规则。
func paintLinearGradient(dst *image.RGBA, r ui.Rect, scale float32, from, to ui.Color, angle, start float32) {
	a := float64(angle) * math.Pi / 180
	dx, dy := float32(math.Sin(a)), -float32(math.Cos(a))
	half := (r.W*absF(dx) + r.H*absF(dy)) / 2
	if half <= 0 {
		return
	}
	cx, cy := r.W*scale/2, r.H*scale/2
	sx, sy := cx-dx*half*scale, cy-dy*half*scale
	ex, ey := sx+dx*2*half*scale, sy+dy*2*half*scale
	sx += (ex - sx) * start
	sy += (ey - sy) * start
	stepX, stepY := ex-sx, ey-sy
	denominator := stepX*stepX + stepY*stepY
	if denominator <= 0 {
		return
	}
	for y := 0; y < dst.Bounds().Dy(); y++ {
		py := float32(y) + .5 - sy
		for x := 0; x < dst.Bounds().Dx(); x++ {
			px := float32(x) + .5 - sx
			t := (px*stepX + py*stepY) / denominator
			t = min(max(t, 0), 1)
			alpha := float32(from.A)/255*(1-t) + float32(to.A)/255*t
			blendPixel(dst, x, y, from, to, t, alpha)
		}
	}
}

func blendPixel(dst *image.RGBA, x, y int, from, to ui.Color, t, alpha float32) {
	i := dst.PixOffset(x, y)
	before := [3]float32{float32(dst.Pix[i]) / 255, float32(dst.Pix[i+1]) / 255, float32(dst.Pix[i+2]) / 255}
	oldAlpha := float32(dst.Pix[i+3]) / 255
	outAlpha := alpha + oldAlpha*(1-alpha)
	for c, rgb := range [3][2]uint8{{from.R, to.R}, {from.G, to.G}, {from.B, to.B}} {
		straight := float32(rgb[0])/255*(1-t) + float32(rgb[1])/255*t
		premultiplied := straight * alpha
		if outAlpha > 1e-6 {
			premultiplied /= outAlpha
		}
		dst.Pix[i+c] = unitByte(premultiplied*outAlpha + before[c]*(1-alpha))
	}
	dst.Pix[i+3] = unitByte(outAlpha)
}

func paintRadialGlow(dst *image.RGBA, cx, cy, radius float32, c ui.Color) {
	if radius <= 0 || c.A == 0 {
		return
	}
	bounds := image.Rect(
		max(0, int(math.Floor(float64(cx-radius)))),
		max(0, int(math.Floor(float64(cy-radius)))),
		min(dst.Bounds().Dx(), int(math.Ceil(float64(cx+radius)))),
		min(dst.Bounds().Dy(), int(math.Ceil(float64(cy+radius)))),
	)
	zero := ui.Color{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		dy := float32(y) + .5 - cy
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float32(x) + .5 - cx
			t := float32(math.Sqrt(float64(dx*dx+dy*dy))) / radius
			if t < 1 {
				blendPixel(dst, x, y, c, zero, t, float32(c.A)/255*(1-t))
			}
		}
	}
}

type bitmapPoint struct{ x, y float32 }

// paintStroke 把整条折线作为一个闭合多边形绘制，而不是逐段叠加透明色。
// 这样光带曲线上的接缝不会因重复混合而变亮。
func paintStroke(dst *image.RGBA, pts []bitmapPoint, width float32, c ui.Color) {
	if len(pts) < 2 || width <= 0 || c.A == 0 {
		return
	}
	half := width / 2
	left := make([]bitmapPoint, len(pts))
	right := make([]bitmapPoint, len(pts))
	for i, p := range pts {
		from, to := p, p
		if i > 0 {
			from = pts[i-1]
		}
		if i+1 < len(pts) {
			to = pts[i+1]
		}
		dx, dy := to.x-from.x, to.y-from.y
		length := float32(math.Hypot(float64(dx), float64(dy)))
		if length == 0 {
			dx, length = 1, 1
		}
		nx, ny := -dy/length*half, dx/length*half
		left[i] = bitmapPoint{p.x + nx, p.y + ny}
		right[i] = bitmapPoint{p.x - nx, p.y - ny}
	}
	outline := append(make([]bitmapPoint, 0, 2*len(pts)+20), left...)
	addCap := func(p bitmapPoint, angle, direction float32) {
		const segments = 12
		for j := 1; j <= segments; j++ {
			a := angle + direction*math.Pi*float32(j)/segments
			outline = append(outline, bitmapPoint{p.x + half*float32(math.Cos(float64(a))), p.y + half*float32(math.Sin(float64(a)))})
		}
	}
	last := pts[len(pts)-1]
	endX := math.Atan2(float64(left[len(left)-1].y-last.y), float64(left[len(left)-1].x-last.x))
	addCap(last, float32(endX), math.Pi)
	for _, r := range slices.Backward(right) {
		outline = append(outline, r)
	}
	first := pts[0]
	startX := math.Atan2(float64(right[0].y-first.y), float64(right[0].x-first.x))
	addCap(first, float32(startX), -math.Pi)
	paintPolygon(dst, outline, c)
}

func paintPolygon(dst *image.RGBA, pts []bitmapPoint, c ui.Color) {
	if len(pts) < 3 || c.A == 0 {
		return
	}
	box := bitmapBounds(dst.Bounds(), pts, 1)
	if box.Empty() {
		return
	}
	z := vector.NewRasterizer(box.Dx(), box.Dy())
	z.MoveTo(pts[0].x-float32(box.Min.X), pts[0].y-float32(box.Min.Y))
	for _, p := range pts[1:] {
		z.LineTo(p.x-float32(box.Min.X), p.y-float32(box.Min.Y))
	}
	z.ClosePath()
	sub := &image.RGBA{
		Pix:    dst.Pix[dst.PixOffset(box.Min.X, box.Min.Y):],
		Stride: dst.Stride,
		Rect:   image.Rectangle{Max: box.Size()},
	}
	z.Draw(sub, sub.Bounds(), image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}), image.Point{})
}

func bitmapBounds(bounds image.Rectangle, pts []bitmapPoint, padding float32) image.Rectangle {
	minX, minY, maxX, maxY := pts[0].x, pts[0].y, pts[0].x, pts[0].y
	for _, p := range pts[1:] {
		minX, maxX = min(minX, p.x), max(maxX, p.x)
		minY, maxY = min(minY, p.y), max(maxY, p.y)
	}
	r := image.Rect(
		int(math.Floor(float64(minX-padding))), int(math.Floor(float64(minY-padding))),
		int(math.Ceil(float64(maxX+padding))), int(math.Ceil(float64(maxY+padding))),
	)
	return r.Intersect(bounds)
}

func paintEllipse(dst *image.RGBA, cx, cy, rx, ry float32, c ui.Color) {
	pts := make([]bitmapPoint, 16)
	for i := range pts {
		a := 2 * math.Pi * float64(i) / float64(len(pts))
		pts[i] = bitmapPoint{cx + rx*float32(math.Cos(a)), cy + ry*float32(math.Sin(a))}
	}
	paintPolygon(dst, pts, c)
}

func paintLine(dst *image.RGBA, x0, y0, x1, y1, width float32, c ui.Color) {
	r := image.Rect(
		deviceLength(min(x0, x1), 1), deviceLength(min(y0, y1), 1),
		deviceLength(max(x0, x1), 1), deviceLength(max(y0, y1), 1),
	)
	if r.Dx() < r.Dy() {
		r.Min.X -= deviceLength(width/2, 1)
		r.Max.X = r.Min.X + deviceLength(width, 1)
	} else {
		r.Min.Y -= deviceLength(width/2, 1)
		r.Max.Y = r.Min.Y + deviceLength(width, 1)
	}
	draw.Draw(dst, r.Intersect(dst.Bounds()), image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}), image.Point{}, draw.Over)
}

// drawStars 保持固定随机种子，缓存重建或切换屏幕倍率时星点不会闪烁。
func drawStars(dst *image.RGBA, r ui.Rect, scale float32, maxY float32) {
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
		x, y := nx*r.W*scale, ny*r.H*scale
		size := (.65 + next()*1.35) * scale
		a := .20 + next()*.56
		x, y, size = float32(deviceLength(x, 1)), float32(deviceLength(y, 1)), float32(deviceLength(size, 1))
		paintEllipse(dst, x+size/2, y+size/2, size/2, size/2, ui.RGBA(224, 248, 255, a))
		if i%29 == 0 {
			arm := size * 3.4
			width := .7 * scale
			paintLine(dst, x-arm, y+size/2, x+arm, y+size/2, width, ui.RGBA(207, 243, 255, a*.55))
			paintLine(dst, x+size/2, y-arm, x+size/2, y+arm, width, ui.RGBA(207, 243, 255, a*.55))
		}
	}
}

func auroraCurtain(dst *image.RGBA, r ui.Rect, scale float32, x0, x1, baseY, amplitude, phase, frequency, width float32, color ui.Color, strength float32) {
	const steps = 52
	center := make([]bitmapPoint, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := float32(i) / steps
		wave := amplitude*float32(math.Sin(float64(t*frequency*2*math.Pi+phase))) +
			amplitude*.34*float32(math.Sin(float64(t*frequency*5.1*math.Pi+phase*1.9)))
		center = append(center, bitmapPoint{
			x: r.W * (x0 + (x1-x0)*t) * scale,
			y: r.H * (baseY + wave) * scale,
		})
	}
	for _, layer := range []struct{ width, alpha float32 }{
		{width * 1.35, .030 * strength},
		{width * .94, .048 * strength},
		{width * .62, .075 * strength},
		{width * .34, .115 * strength},
		{width * .16, .18 * strength},
		{width * .05, .28 * strength},
	} {
		paintStroke(dst, center, layer.width*scale, color.Alpha(layer.alpha))
	}
}

func unitByte(v float32) uint8 {
	return uint8(min(max(v*255+.5, 0), 255))
}

func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
