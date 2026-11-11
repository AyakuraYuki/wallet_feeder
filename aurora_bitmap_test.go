package main

import "testing"

// 极光背景必须直接覆盖当前设备的全部像素，而不是在 Retina 下反复占用字形图集。
// 不变窗口的重复绘制复用位图，变化尺寸或倍率才重新生成。
func TestAuroraBackdropUsesDevicePixelsAndCaches(t *testing.T) {
	a := NewApp(nil)
	t.Cleanup(a.close)
	a.useGlass = true
	tt := testerFor(a)
	if a.backdrop == nil {
		t.Fatal("首个极光背景没有生成位图")
	}
	first := a.backdrop
	tt.SetScale(2)
	if a.backdrop == first {
		t.Fatal("切换到 Retina 后仍在使用 1× 的背景")
	}
	if a.backdrop.width != 2240 || a.backdrop.height != 1560 {
		t.Fatalf("Retina 背景尺寸为 %d×%d，应为 2240×1560", a.backdrop.width, a.backdrop.height)
	}
	if a.backdrop.scale != 2 {
		t.Fatalf("背景倍率为 %g，应为 2", a.backdrop.scale)
	}
	retina := a.backdrop
	tt.Frame()
	if a.backdrop != retina {
		t.Fatal("窗口尺寸和倍率未变时重新生成了背景")
	}
	tt.SetSize(1000, 760)
	if a.backdrop == retina {
		t.Fatal("窗口大小变化后没有重新生成背景")
	}
	if a.backdrop.width != 2000 || a.backdrop.height != 1520 {
		t.Fatalf("缩放后的背景尺寸为 %d×%d，应为 2000×1520", a.backdrop.width, a.backdrop.height)
	}
}
