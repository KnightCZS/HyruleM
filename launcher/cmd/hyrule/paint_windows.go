//go:build windows

package main

import "unsafe"

// Paint once into an offscreen bitmap. Background erases and per-control moves
// previously exposed partially drawn frames while resizing or scrolling.
func (a *app) paintBuffered(screen uintptr) {
	paintWindowBuffered(a.window, screen, a.paint)
}
func paintWindowBuffered(window, screen uintptr, paint func(uintptr)) {
	var r rect
	getClientRect.Call(window, uintptr(unsafe.Pointer(&r)))
	if r.Right <= 0 || r.Bottom <= 0 {
		return
	}
	dc, _, _ := compatibleDC.Call(screen)
	bitmap, _, _ := compatibleBitmap.Call(screen, uintptr(r.Right), uintptr(r.Bottom))
	if dc == 0 || bitmap == 0 {
		if dc != 0 {
			deleteDC.Call(dc)
		}
		if bitmap != 0 {
			deleteObject.Call(bitmap)
		}
		paint(screen)
		return
	}
	old, _, _ := selectObject.Call(dc, bitmap)
	paint(dc)
	setViewport.Call(dc, 0, 0, 0)
	bitBlt.Call(screen, 0, 0, uintptr(r.Right), uintptr(r.Bottom), dc, 0, 0, 0x00cc0020)
	selectObject.Call(dc, old)
	deleteObject.Call(bitmap)
	deleteDC.Call(dc)
}
