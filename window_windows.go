package main

import (
	"clothesline/internal/shelf"
	"runtime"
	"sync"
	"unsafe"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

var hotkeyLabels = [2]string{"Ctrl+Alt+A", "Ctrl+Alt+S"}

var (
	windowUser        = windows.NewLazySystemDLL("user32.dll")
	windowFind        = windowUser.NewProc("FindWindowExW")
	windowOwner       = windowUser.NewProc("GetWindowThreadProcessId")
	windowGetStyle    = windowUser.NewProc("GetWindowLongPtrW")
	windowSetStyle    = windowUser.NewProc("SetWindowLongPtrW")
	windowShow        = windowUser.NewProc("ShowWindow")
	windowVisible     = windowUser.NewProc("IsWindowVisible")
	windowSetPos      = windowUser.NewProc("SetWindowPos")
	windowMonitor     = windowUser.NewProc("MonitorFromWindow")
	windowMonitorRect = windowUser.NewProc("MonitorFromRect")
	windowMonitorInfo = windowUser.NewProc("GetMonitorInfoW")
	windowMonitorDPI  = windows.NewLazySystemDLL("shcore.dll").NewProc("GetDpiForMonitor")
)

type windowRect struct{ Left, Top, Right, Bottom int32 }
type windowMonitorData struct {
	Size          uint32
	Monitor, Work windowRect
	Flags         uint32
}

func mainWindowHandle() uintptr {
	class, _ := windows.UTF16PtrFromString("wailsWindow")
	for after := uintptr(0); ; {
		hwnd, _, _ := windowFind.Call(0, after, uintptr(unsafe.Pointer(class)), 0)
		if hwnd == 0 {
			return 0
		}
		var pid uint32
		windowOwner.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == windows.GetCurrentProcessId() {
			return hwnd
		}
		after = hwnd
	}
}

// Apply before the initially hidden window is shown. Wails creates APPWINDOW
// by default; TOOLWINDOW keeps both the shelf and basket off the taskbar.
func platformWindowSetup(a *App) {
	hwnd := mainWindowHandle()
	if hwnd == 0 {
		a.notice("无法设置托盘常驻模式")
		return
	}
	visible, _, _ := windowVisible.Call(hwnd)
	if visible != 0 {
		windowShow.Call(hwnd, 0)
	}
	index := ^uintptr(19) // GWL_EXSTYLE = -20
	style, _, _ := windowGetStyle.Call(hwnd, index)
	windowSetStyle.Call(hwnd, index, (style&^0x00040000)|0x00000080)
	// SWP_NOSIZE | SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE | FRAMECHANGED
	windowSetPos.Call(hwnd, 0, 0, 0, 0, 0, 0x0037)
	if visible != 0 {
		windowShow.Call(hwnd, 4)
	}
}
func showWindowQuietly(a *App) {
	if hwnd := mainWindowHandle(); hwnd != 0 {
		windowShow.Call(hwnd, 4 /* SW_SHOWNOACTIVATE */)
	} else {
		wr.WindowShow(a.ctx)
	}
}

// Wails gets absolute virtual-desktop coordinates but its position setter adds
// the current monitor's work-area origin. Restore the basket using Win32 on both
// sides, including monitors left of the primary and taskbars on any edge.
func placeBasket(a *App, saved *shelf.Point) {
	hwnd := mainWindowHandle()
	if hwnd == 0 {
		return
	}
	monitor, _, _ := windowMonitor.Call(hwnd, 2 /* NEAREST */)
	if saved != nil {
		r := windowRect{int32(saved.X), int32(saved.Y), int32(saved.X + 80), int32(saved.Y + 80)}
		monitor, _, _ = windowMonitorRect.Call(uintptr(unsafe.Pointer(&r)), 2)
	}
	info := windowMonitorData{}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := windowMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return
	}
	dpi := uint32(96)
	if windowMonitorDPI.Find() == nil {
		var dx, dy uint32
		if hr, _, _ := windowMonitorDPI.Call(monitor, 0, uintptr(unsafe.Pointer(&dx)), uintptr(unsafe.Pointer(&dy))); hr == 0 && dx > 0 {
			dpi = dx
		}
	}
	size := (80*int(dpi) + 48) / 96
	area := shelf.Frame{X: int(info.Work.Left), Y: int(info.Work.Top), W: int(info.Work.Right - info.Work.Left), H: int(info.Work.Bottom - info.Work.Top)}
	p := shelf.Point{X: area.X + area.W - size - 16, Y: area.Y + area.H - size - 24}
	if saved != nil {
		p = *saved
	}
	p = clampBasket(p, area, size, size)
	windowSetPos.Call(hwnd, 0, uintptr(p.X), uintptr(p.Y), uintptr(size), uintptr(size), 0x0014 /* NOZORDER | NOACTIVATE */)
}

// Edge resizing is driven by the page on Windows; the frontend turns it on only
// for the shelf.
func setResizable(*App, bool) {}
func workArea(a *App) (int, int) {
	if ss, err := wr.ScreenGetAll(a.ctx); err == nil {
		for _, current := range []bool{true, false} {
			for _, s := range ss {
				if (current && s.IsCurrent) || (!current && s.IsPrimary) {
					return s.Size.Width, s.Size.Height
				}
			}
		}
	}
	return 1440, 900
}
func mainScreenSize(a *App) (int, int) { return a.screenWidth(), a.screenHeight() }
func placeWindow(a *App, x, y, w, h int, _ bool) {
	wr.WindowSetSize(a.ctx, w, h)
	wr.WindowSetPosition(a.ctx, x, y)
}

var (
	hotkeyUser        = windows.NewLazySystemDLL("user32.dll")
	hotkeyRegister    = hotkeyUser.NewProc("RegisterHotKey")
	hotkeyUnregister  = hotkeyUser.NewProc("UnregisterHotKey")
	hotkeyGetMessage  = hotkeyUser.NewProc("GetMessageW")
	hotkeyPostMessage = hotkeyUser.NewProc("PostThreadMessageW")
	hotkeyPeekMessage = hotkeyUser.NewProc("PeekMessageW")
	hotkeyMu          sync.Mutex
	hotkeyThread      uint32
	hotkeyDone        chan struct{}
)

type hotkeyMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	X, Y    int32
	Private uint32
}

// setHotkeys registers the shortcuts on a dedicated thread: WM_HOTKEY is posted
// to the queue of the thread that registered them.
func setHotkeys(a *App, enabled bool) []string {
	hotkeyMu.Lock()
	defer hotkeyMu.Unlock()
	if hotkeyThread != 0 {
		hotkeyPostMessage.Call(uintptr(hotkeyThread), 0x0012 /* WM_QUIT */, 0, 0)
		<-hotkeyDone
		hotkeyThread = 0
	}
	if !enabled {
		return nil
	}
	result := make(chan []string, 1)
	hotkeyDone = make(chan struct{})
	ready := make(chan uint32, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(hotkeyDone)
		var taken []string
		for i, key := range []uintptr{'A', 'S'} {
			// MOD_ALT | MOD_CONTROL | MOD_NOREPEAT
			if ok, _, _ := hotkeyRegister.Call(0, uintptr(i+1), 0x0001|0x0002|0x4000, key); ok == 0 {
				taken = append(taken, hotkeyLabels[i])
			}
		}
		var msg hotkeyMsg
		// Creates the thread's queue so the WM_QUIT posted on shutdown is not lost.
		hotkeyPeekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0x0400, 0x0400, 0)
		ready <- windows.GetCurrentThreadId()
		result <- taken
		for {
			r, _, _ := hotkeyGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			if msg.Message == 0x0312 /* WM_HOTKEY */ {
				go a.hotkey(int(msg.WParam))
			}
		}
		hotkeyUnregister.Call(0, 1)
		hotkeyUnregister.Call(0, 2)
	}()
	hotkeyThread = <-ready
	return <-result
}
