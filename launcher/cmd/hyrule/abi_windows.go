//go:build windows && amd64

package main

import "unsafe"

// A mismatch in these Win32 ABI layouts must fail the build, not the file dialog.
var _ [80 - unsafe.Sizeof(wndClass{})]byte
var _ [unsafe.Sizeof(wndClass{}) - 80]byte
var _ [48 - unsafe.Sizeof(msg{})]byte
var _ [unsafe.Sizeof(msg{}) - 48]byte
var _ [152 - unsafe.Sizeof(openFileName{})]byte
var _ [unsafe.Sizeof(openFileName{}) - 152]byte
var _ [64 - unsafe.Sizeof(browseInfo{})]byte
var _ [unsafe.Sizeof(browseInfo{}) - 64]byte
var _ [64 - unsafe.Sizeof(drawItem{})]byte
var _ [unsafe.Sizeof(drawItem{}) - 64]byte
var _ [72 - unsafe.Sizeof(paintStruct{})]byte
var _ [unsafe.Sizeof(paintStruct{}) - 72]byte
