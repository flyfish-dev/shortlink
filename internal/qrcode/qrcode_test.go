package qrcode

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"

	"github.com/makiuchi-d/gozxing"
	zxingqr "github.com/makiuchi-d/gozxing/qrcode"
)

func TestStyledPNGWithCenterLogoDecodes(t *testing.T) {
	content := "https://s.flyfish.dev/q/live-demo-code"
	logo := image.NewRGBA(image.Rect(0, 0, 96, 72))
	draw.Draw(logo, logo.Bounds(), image.Transparent, image.Point{}, draw.Src)
	drawRoundedTestMark(logo, image.Rect(12, 8, 84, 64), color.RGBA{37, 99, 235, 255})
	drawRoundedTestMark(logo, image.Rect(34, 24, 62, 52), color.RGBA{255, 255, 255, 255})

	pngBytes, err := StyledPNG(content, Options{
		Scale:      10,
		Border:     4,
		Shape:      "rounded",
		Foreground: "#111827",
		Background: "#ffffff",
		LogoURL:    "/uploads/test-logo.png",
	}, logo)
	if err != nil {
		t.Fatalf("StyledPNG returned error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("generated PNG did not decode: %v", err)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatalf("NewBinaryBitmapFromImage returned error: %v", err)
	}
	result, err := zxingqr.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("generated QR with center logo did not scan: %v", err)
	}
	if got := result.GetText(); got != content {
		t.Fatalf("decoded content = %q, want %q", got, content)
	}
}

func drawRoundedTestMark(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	radius := max(4, min(r.Dx(), r.Dy())/5)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dx := min(x-r.Min.X, r.Max.X-1-x)
			dy := min(y-r.Min.Y, r.Max.Y-1-y)
			if dx >= radius || dy >= radius {
				dst.SetRGBA(x, y, c)
				continue
			}
			cx := radius - dx
			cy := radius - dy
			if cx*cx+cy*cy <= radius*radius {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}
