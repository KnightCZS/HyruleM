//go:build windows

package main

import (
	mp "hyrulelauncher/internal/multiplayer"
	"math"
	"syscall"
	"time"
	"unsafe"
)

var (
	enumWindows    = user32.NewProc("EnumWindows")
	windowPID      = user32.NewProc("GetWindowThreadProcessId")
	isVisible      = user32.NewProc("IsWindowVisible")
	isIconic       = user32.NewProc("IsIconic")
	clientToScreen = user32.NewProc("ClientToScreen")
	layered        = user32.NewProc("SetLayeredWindowAttributes")
	polygon        = gdi32.NewProc("Polygon")
	stockObject    = gdi32.NewProc("GetStockObject")
	getWindowRect  = user32.NewProc("GetWindowRect")
)

var compatibleDC = gdi32.NewProc("CreateCompatibleDC")
var compatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
var bitBlt = gdi32.NewProc("BitBlt")
var deleteDC = gdi32.NewProc("DeleteDC")

const transparentKey = 0x00030201

// Go's Windows callbacks are never reclaimed: allocate one callback, not one
// per frame. Enumeration runs only on the launcher's locked UI thread.
var enumTarget int
var enumBest uintptr
var enumArea int64
var enumCallback = syscall.NewCallback(func(h, l uintptr) uintptr {
	var p uint32
	windowPID.Call(h, uintptr(unsafe.Pointer(&p)))
	if int(p) != enumTarget {
		return 1
	}
	visible, _, _ := isVisible.Call(h)
	if visible == 0 {
		return 1
	}
	var r rect
	getClientRect.Call(h, uintptr(unsafe.Pointer(&r)))
	area := int64(r.Right) * int64(r.Bottom)
	if r.Right > 300 && r.Bottom > 180 && area > enumArea {
		enumBest = h
		enumArea = area
	}
	return 1
})

func findGameWindow(pid int) uintptr {
	if pid <= 0 {
		return 0
	}
	enumTarget = pid
	enumBest = 0
	enumArea = 0
	enumWindows.Call(enumCallback, 0)
	return enumBest
}
func (u *multiplayerUI) tick() {
	for i := range u.slots {
		s := &u.slots[i]
		if s.bridge == nil {
			if s.overlay != 0 {
				showWindow.Call(s.overlay, 0)
			}
			continue
		}
		if s.game == 0 {
			s.game = findGameWindow(s.pid)
		}
		if s.game == 0 {
			continue
		}
		var pid uint32
		windowPID.Call(s.game, uintptr(unsafe.Pointer(&pid)))
		visible, _, _ := isVisible.Call(s.game)
		mini, _, _ := isIconic.Call(s.game)
		if int(pid) != s.pid {
			if s.overlay != 0 {
				destroyWindow.Call(s.overlay)
				s.overlay = 0
			}
			s.game = 0
			continue
		}
		if mini != 0 || visible == 0 {
			if s.overlay != 0 {
				showWindow.Call(s.overlay, 0)
			}
			continue
		}
		if s.overlay == 0 {
			s.overlay, _, _ = createWindow.Call(0x80000|0x20|0x80|0x08000000, uintptr(unsafe.Pointer(wide("HyruleRemoteAvatar"))), uintptr(unsafe.Pointer(wide("HyruleM avatar"))), 0x80000000, 0, 0, 1, 1, s.game, 0, u.a.instance, 0)
			if s.overlay == 0 {
				continue
			}
			layered.Call(s.overlay, transparentKey, 255, 1)
		}
		var r rect
		getClientRect.Call(s.game, uintptr(unsafe.Pointer(&r)))
		p := point{}
		clientToScreen.Call(s.game, uintptr(unsafe.Pointer(&p)))
		if r.Right < 1 || r.Bottom < 1 {
			continue
		}
		setWindowPos.Call(s.overlay, 0, uintptr(p.X), uintptr(p.Y), uintptr(r.Right), uintptr(r.Bottom), 0x10|0x40|0x4|0x200)
		invalidate.Call(s.overlay, 0, 0)
	}
}
func overlayProc(h uintptr, m uint32, w, l uintptr) uintptr {
	if m == 0x84 {
		return ^uintptr(0)
	} // HTTRANSPARENT
	if m == 0x21 {
		return 3
	} // MA_NOACTIVATE
	if m == 0x14 {
		return 1
	}
	if m != wmPaint {
		return defaultProc(h, m, w, l)
	}
	var ps paintStruct
	dc, _, _ := beginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
	defer endPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
	var r rect
	getClientRect.Call(h, uintptr(unsafe.Pointer(&r)))
	// Buffer GDI drawing so color-key clears cannot flash over the game.
	screenDC := dc
	memoryDC, _, _ := compatibleDC.Call(screenDC)
	bitmap, _, _ := compatibleBitmap.Call(screenDC, uintptr(r.Right), uintptr(r.Bottom))
	if memoryDC != 0 && bitmap != 0 {
		old, _, _ := selectObject.Call(memoryDC, bitmap)
		dc = memoryDC
		defer func() {
			bitBlt.Call(screenDC, 0, 0, uintptr(r.Right), uintptr(r.Bottom), memoryDC, 0, 0, 0x00cc0020)
			selectObject.Call(memoryDC, old)
			deleteObject.Call(bitmap)
			deleteDC.Call(memoryDC)
		}()
	} else {
		if bitmap != 0 {
			deleteObject.Call(bitmap)
		}
		if memoryDC != 0 {
			deleteDC.Call(memoryDC)
		}
	}
	bg, _, _ := createBrush.Call(transparentKey)
	fillRect.Call(dc, uintptr(unsafe.Pointer(&r)), bg)
	deleteObject.Call(bg)
	a := current
	if a == nil || a.multiplayer == nil {
		return 0
	}
	var slot *mpSlot
	for i := range a.multiplayer.slots {
		if a.multiplayer.slots[i].overlay == h {
			slot = &a.multiplayer.slots[i]
			break
		}
	}
	if slot == nil || slot.bridge == nil {
		return 0
	}
	camera := slot.bridge.Camera()
	now := time.Now()
	list := mp.BuildDrawList(camera, slot.bridge.Client.Remote(now), int(r.Right), int(r.Bottom), now)
	nullPen, _, _ := stockObject.Call(8)
	oldPen, _, _ := selectObject.Call(dc, nullPen)
	defer selectObject.Call(dc, oldPen)
	brushes := map[mp.Color]uintptr{}
	defer func() {
		for _, b := range brushes {
			deleteObject.Call(b)
		}
	}()
	for _, tri := range list.Triangles {
		b := brushes[tri.Color]
		if b == 0 {
			b, _, _ = createBrush.Call(rgb(tri.Color.R, tri.Color.G, tri.Color.B))
			brushes[tri.Color] = b
		}
		old, _, _ := selectObject.Call(dc, b)
		var p [3]point
		for i, v := range tri.Points {
			p[i] = point{int32(math.Round(v.X)), int32(math.Round(v.Y))}
		}
		polygon.Call(dc, uintptr(unsafe.Pointer(&p[0])), 3)
		selectObject.Call(dc, old)
	}
	for _, l := range list.Labels {
		x, y := int32(l.Point.X), int32(l.Point.Y)
		a.label(dc, l.Name, "button", rgb(l.Color.R, l.Color.G, l.Color.B), rect{x - 150, y - 25, x + 150, y}, 0x21)
	}
	return 0
}
func (u *multiplayerUI) arrange() {
	var work rect
	user32.NewProc("SystemParametersInfoW").Call(48, 0, uintptr(unsafe.Pointer(&work)), 0)
	width := (work.Right - work.Left) / 2

	for i := range u.slots {
		s := &u.slots[i]
		if s.game == 0 {
			s.game = findGameWindow(s.pid)
		}
		if s.game == 0 {
			continue
		}
		showWindow.Call(s.game, 9)
		var outer, client rect
		getWindowRect.Call(s.game, uintptr(unsafe.Pointer(&outer)))
		getClientRect.Call(s.game, uintptr(unsafe.Pointer(&client)))
		chromeW := outer.Right - outer.Left - client.Right
		chromeH := outer.Bottom - outer.Top - client.Bottom
		h := (width-chromeW)*9/16 + chromeH
		if h > work.Bottom-work.Top {
			h = work.Bottom - work.Top
		}
		setWindowPos.Call(s.game, 0, uintptr(work.Left+int32(i)*width), uintptr(work.Top), uintptr(width), uintptr(h), 0x4|0x10)
	}
}
