package main

/*
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#cgo CFLAGS: -x objective-c -fobjc-arc
#include <stdlib.h>
void clTrayStart(const void *bytes, int len);
void clTrayStop(void);
void clSetApplicationIcon(const void *bytes, int len);
void clSetDockVisible(int visible);
void clBeginDrag(const char *path,const void *text,int textLen,const void *thumb,int thumbLen);
*/
import "C"
import (
	"clothesline/internal/shelf"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"unsafe"
)

var trayApp *App

func setupTray(a *App) func() {
	trayApp = a
	appIcon, _ := assets.ReadFile("frontend/appicon.png")
	color := C.CBytes(appIcon)
	C.clSetApplicationIcon(color, C.int(len(appIcon)))
	C.free(color)
	data, _ := assets.ReadFile("frontend/tray.png")
	p := C.CBytes(data)
	C.clTrayStart(p, C.int(len(data)))
	C.free(p)
	return func() { C.clTrayStop() }
}
func applyPlatformSettings(v shelf.Settings) {
	visible := 0
	if v.ShowDockIcon {
		visible = 1
	}
	C.clSetDockVisible(C.int(visible))
}

//export clTrayAction
func clTrayAction(action C.int) {
	go func() {
		a := trayApp
		if a == nil {
			return
		}
		switch int(action) {
		case 1:
			a.Expand()
		case 2:
			if e := a.Capture(); e != nil {
				a.notice(e.Error())
			}
		case 3:
			if e := a.Paste(); e != nil {
				a.notice(e.Error())
			}
		case 4:
			a.Expand()
			wr.EventsEmit(a.ctx, "compose")
		case 5:
			a.Quit()
		}
	}()
}

var _ unsafe.Pointer

func startNativeDrag(path, text string, thumb []byte) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	t := C.CBytes([]byte(text))
	defer C.free(t)
	b := C.CBytes(thumb)
	defer C.free(b)
	C.clBeginDrag(p, t, C.int(len([]byte(text))), b, C.int(len(thumb)))
	return nil
}

//export clDragEnded
func clDragEnded() {
	go func() {
		if trayApp != nil {
			trayApp.dragEnded()
		}
	}()
}

//export clDragStarted
func clDragStarted() {
	go func() {
		if trayApp == nil {
			return
		}
		trayApp.mu.Lock()
		active := trayApp.dragging
		trayApp.mu.Unlock()
		if active {
			wr.EventsEmit(trayApp.ctx, "drag-started")
		}
	}()
}
