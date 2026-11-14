//go:build windows

package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

// Capture only the synthetic test window, including actual native child controls.
// PrintWindow -> WM_PRINTCLIENT also exercises the buffered paint path used live.
func captureReview(t *testing.T, window uintptr, path string) {
	t.Helper()
	var r rect
	getClientRect.Call(window, uintptr(unsafe.Pointer(&r)))
	w, h := int(r.Right), int(r.Bottom)
	if w < 1 || h < 1 {
		t.Fatal("empty review window")
	}
	dc, _, _ := user32.NewProc("GetDC").Call(window)
	defer user32.NewProc("ReleaseDC").Call(window, dc)
	memory, _, _ := compatibleDC.Call(dc)
	defer deleteDC.Call(memory)
	bitmap, _, _ := compatibleBitmap.Call(dc, uintptr(w), uintptr(h))
	if bitmap == 0 {
		t.Fatal("review bitmap allocation")
	}
	defer deleteObject.Call(bitmap)
	old, _, _ := selectObject.Call(memory, bitmap)
	ok, _, _ := user32.NewProc("PrintWindow").Call(window, memory, 1)
	selectObject.Call(memory, old)
	if ok == 0 {
		t.Fatal("PrintWindow failed")
	}
	type header struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		Used, Important        uint32
	}
	info := header{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	pixels := make([]byte, w*h*4)
	lines, _, _ := gdi32.NewProc("GetDIBits").Call(dc, bitmap, 0, uintptr(h), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), 0)
	if int(lines) != h {
		t.Fatal("review readback failed", lines)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(pixels); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pixels[i+2], pixels[i+1], pixels[i], 255
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
}
