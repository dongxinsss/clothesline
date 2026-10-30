package main

// Native OLE file/text drag-and-drop. The system clipboard is not modified.
import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dragFormat struct {
	Format       uint16
	TargetDevice uintptr
	Aspect       uint32
	Index        int32
	Medium       uint32
}
type dragMedium struct {
	Medium  uint32
	Handle  uintptr
	Release uintptr
}
type dragCOM struct {
	Table *uintptr
	Refs  atomic.Int32
	Kind  uint32
}
type dragDataObject struct {
	dragCOM
	Format  dragFormat
	Payload []byte
}
type dragFormatEnum struct {
	dragCOM
	Format dragFormat
	Used   bool
}

var dragObjects = struct {
	sync.Mutex
	items map[uintptr]any
}{items: map[uintptr]any{}}
var dragOle = windows.NewLazySystemDLL("ole32.dll")
var dragKernel = windows.NewLazySystemDLL("kernel32.dll")
var dragOleInit = dragOle.NewProc("OleInitialize")
var dragOleUninit = dragOle.NewProc("OleUninitialize")
var dragDoDrop = dragOle.NewProc("DoDragDrop")
var dragGlobalAlloc = dragKernel.NewProc("GlobalAlloc")
var dragGlobalLock = dragKernel.NewProc("GlobalLock")
var dragGlobalUnlock = dragKernel.NewProc("GlobalUnlock")
var dragGlobalFree = dragKernel.NewProc("GlobalFree")
var dragUser = windows.NewLazySystemDLL("user32.dll")
var dragAsyncKey = dragUser.NewProc("GetAsyncKeyState")
var dragMetrics = dragUser.NewProc("GetSystemMetrics")
var dragFindWindow = dragUser.NewProc("FindWindowExW")
var dragWindowThread = dragUser.NewProc("GetWindowThreadProcessId")
var dragAttachInput = dragUser.NewProc("AttachThreadInput")
var dragDataTable = [12]uintptr{syscall.NewCallback(dragQuery), syscall.NewCallback(dragAddRef), syscall.NewCallback(dragRelease), syscall.NewCallback(dragGetData), syscall.NewCallback(dragNotImpl3), syscall.NewCallback(dragQueryData), syscall.NewCallback(dragCanonical), syscall.NewCallback(dragNotImpl4), syscall.NewCallback(dragEnumFormats), syscall.NewCallback(dragNoAdvise5), syscall.NewCallback(dragNoAdvise2), syscall.NewCallback(dragNoAdvise2)}
var dragSourceTable = [5]uintptr{syscall.NewCallback(dragQuery), syscall.NewCallback(dragAddRef), syscall.NewCallback(dragRelease), syscall.NewCallback(dragContinue), syscall.NewCallback(dragFeedback)}
var dragEnumTable [7]uintptr

func init() {
	dragEnumTable = [7]uintptr{syscall.NewCallback(dragQuery), syscall.NewCallback(dragAddRef), syscall.NewCallback(dragRelease), syscall.NewCallback(dragEnumNext), syscall.NewCallback(dragEnumSkip), syscall.NewCallback(dragEnumReset), syscall.NewCallback(dragEnumClone)}
}

func dragRegister(ptr uintptr, obj any) {
	dragObjects.Lock()
	dragObjects.items[ptr] = obj
	dragObjects.Unlock()
}
func dragQuery(this, iid, out uintptr) uintptr {
	if iid == 0 || out == 0 {
		return 0x80004003
	}
	*(*uintptr)(unsafe.Pointer(out)) = 0
	want := *(*windows.GUID)(unsafe.Pointer(iid))
	base := (*dragCOM)(unsafe.Pointer(this))
	id := uint32(0x10e)
	if base.Kind == 2 {
		id = 0x121
	} else if base.Kind == 3 {
		id = 0x103
	}
	unknown := windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	specific := unknown
	specific.Data1 = id
	if want != unknown && want != specific {
		return 0x80004002
	}
	*(*uintptr)(unsafe.Pointer(out)) = this
	return func() uintptr { dragAddRef(this); return 0 }()
}
func dragAddRef(this uintptr) uintptr { return uintptr((*dragCOM)(unsafe.Pointer(this)).Refs.Add(1)) }
func dragRelease(this uintptr) uintptr {
	n := (*dragCOM)(unsafe.Pointer(this)).Refs.Add(-1)
	if n == 0 {
		dragObjects.Lock()
		delete(dragObjects.items, this)
		dragObjects.Unlock()
	}
	return uintptr(n)
}
func dragQueryData(this, format uintptr) uintptr {
	if format == 0 {
		return 0x80004003
	}
	d := (*dragDataObject)(unsafe.Pointer(this))
	f := (*dragFormat)(unsafe.Pointer(format))
	if f.Format != d.Format.Format || f.Aspect != 1 || f.Medium&1 == 0 {
		return 0x80040064
	}
	return 0
}
func dragGetData(this, format, medium uintptr) uintptr {
	if medium == 0 {
		return 0x80004003
	}
	if hr := dragQueryData(this, format); hr != 0 {
		return hr
	}
	d := (*dragDataObject)(unsafe.Pointer(this))
	h, _, _ := dragGlobalAlloc.Call(0x42, uintptr(len(d.Payload)))
	if h == 0 {
		return 0x8007000e
	}
	p, _, _ := dragGlobalLock.Call(h)
	if p == 0 {
		dragGlobalFree.Call(h)
		return 0x8007000e
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(p)), len(d.Payload)), d.Payload)
	dragGlobalUnlock.Call(h)
	*(*dragMedium)(unsafe.Pointer(medium)) = dragMedium{Medium: 1, Handle: h}
	return 0
}
func dragCanonical(this, in, out uintptr) uintptr {
	if in == 0 || out == 0 {
		return 0x80004003
	}
	*(*dragFormat)(unsafe.Pointer(out)) = *(*dragFormat)(unsafe.Pointer(in))
	(*dragFormat)(unsafe.Pointer(out)).TargetDevice = 0
	return 0x40130
}
func dragNotImpl3(uintptr, uintptr, uintptr) uintptr                    { return 0x80004001 }
func dragNotImpl4(uintptr, uintptr, uintptr, uintptr) uintptr           { return 0x80004001 }
func dragNoAdvise5(uintptr, uintptr, uintptr, uintptr, uintptr) uintptr { return 0x80040003 }
func dragNoAdvise2(uintptr, uintptr) uintptr                            { return 0x80040003 }
func newDragEnum(format dragFormat, used bool) *dragFormatEnum {
	e := &dragFormatEnum{dragCOM: dragCOM{Table: &dragEnumTable[0], Kind: 3}, Format: format, Used: used}
	e.Refs.Store(1)
	dragRegister(uintptr(unsafe.Pointer(e)), e)
	return e
}
func dragEnumFormats(this, direction, out uintptr) uintptr {
	if out == 0 {
		return 0x80004003
	}
	if direction != 1 {
		return 0x80004001
	}
	d := (*dragDataObject)(unsafe.Pointer(this))
	e := newDragEnum(d.Format, false)
	*(*uintptr)(unsafe.Pointer(out)) = uintptr(unsafe.Pointer(e))
	return 0
}
func dragEnumNext(this, count, out, fetched uintptr) uintptr {
	if out == 0 || (fetched == 0 && count != 1) {
		return 0x80070057
	}
	if fetched != 0 {
		*(*uint32)(unsafe.Pointer(fetched)) = 0
	}
	if count == 0 {
		return 0
	}
	e := (*dragFormatEnum)(unsafe.Pointer(this))
	if e.Used {
		return 1
	}
	*(*dragFormat)(unsafe.Pointer(out)) = e.Format
	e.Used = true
	if fetched != 0 {
		*(*uint32)(unsafe.Pointer(fetched)) = 1
	}
	if count == 1 {
		return 0
	}
	return 1
}
func dragEnumSkip(this, count uintptr) uintptr {
	e := (*dragFormatEnum)(unsafe.Pointer(this))
	if count == 0 {
		return 0
	}
	if e.Used {
		return 1
	}
	e.Used = true
	if count == 1 {
		return 0
	}
	return 1
}
func dragEnumReset(this uintptr) uintptr {
	(*dragFormatEnum)(unsafe.Pointer(this)).Used = false
	return 0
}
func dragEnumClone(this, out uintptr) uintptr {
	if out == 0 {
		return 0x80004003
	}
	old := (*dragFormatEnum)(unsafe.Pointer(this))
	e := newDragEnum(old.Format, old.Used)
	*(*uintptr)(unsafe.Pointer(out)) = uintptr(unsafe.Pointer(e))
	return 0
}
func dragContinue(this, escape, keyState uintptr) uintptr {
	if escape != 0 {
		return 0x40101
	}
	// The key state OLE passes belongs to this thread's input queue, which may
	// never have seen the press in the WebView; ask the hardware state instead.
	if keyState&1 == 0 && !dragButtonDown() {
		return 0x40100
	}
	return 0
}

// dragButtonDown reports whether the primary mouse button is physically held.
func dragButtonDown() bool {
	// VK_LBUTTON is the physical left button; with SM_SWAPBUTTON set the
	// primary button is the right one.
	key := uintptr(0x01)
	if swapped, _, _ := dragMetrics.Call(23); swapped != 0 {
		key = 0x02
	}
	state, _, _ := dragAsyncKey.Call(key)
	return state&0x8000 != 0
}

// dragUIThread finds the thread that owns this app's window, whose input queue
// holds the mouse press that started the drag.
func dragUIThread() uint32 {
	class, _ := windows.UTF16PtrFromString("wailsWindow")
	pid := uint32(windows.GetCurrentProcessId())
	for after := uintptr(0); ; {
		hwnd, _, _ := dragFindWindow.Call(0, after, uintptr(unsafe.Pointer(class)), 0)
		if hwnd == 0 {
			return 0
		}
		var owner uint32
		thread, _, _ := dragWindowThread.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner == pid {
			return uint32(thread)
		}
		after = hwnd
	}
}
func dragFeedback(uintptr, uintptr) uintptr { return 0x40102 }
func dragUTF16(text string) []byte {
	words := append(utf16.Encode([]rune(text)), 0)
	out := make([]byte, len(words)*2)
	for n, v := range words {
		binary.LittleEndian.PutUint16(out[n*2:], v)
	}
	return out
}
func startNativeDrag(path, text string, _ []byte) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := dragOleInit.Call(0)
	if uint32(hr) > 1 {
		return fmt.Errorf("系统拖放服务未能初始化（%08x）", uint32(hr))
	}
	defer dragOleUninit.Call()
	// DoDragDrop runs on this goroutine's own thread. Sharing the window
	// thread's input lets it take the mouse capture and see the release.
	if ui, self := dragUIThread(), windows.GetCurrentThreadId(); ui != 0 && ui != self {
		if ok, _, _ := dragAttachInput.Call(uintptr(self), uintptr(ui), 1); ok != 0 {
			defer dragAttachInput.Call(uintptr(self), uintptr(ui), 0)
		}
	}
	// A quick flick can end before the call arrives; DoDragDrop would then
	// wait for a release that already happened.
	if !dragButtonDown() {
		return nil
	}
	format := uint16(13)
	payload := dragUTF16(text)
	if path != "" {
		format = 15
		name := append(dragUTF16(path), 0, 0)
		payload = make([]byte, 20+len(name))
		binary.LittleEndian.PutUint32(payload, 20)
		binary.LittleEndian.PutUint32(payload[16:], 1)
		copy(payload[20:], name)
	}
	d := &dragDataObject{dragCOM: dragCOM{Table: &dragDataTable[0], Kind: 1}, Format: dragFormat{Format: format, Aspect: 1, Index: -1, Medium: 1}, Payload: payload}
	d.Refs.Store(1)
	dp := uintptr(unsafe.Pointer(d))
	dragRegister(dp, d)
	defer dragRelease(dp)
	source := &dragCOM{Table: &dragSourceTable[0], Kind: 2}
	source.Refs.Store(1)
	sp := uintptr(unsafe.Pointer(source))
	dragRegister(sp, source)
	defer dragRelease(sp)
	var effect uint32
	hr, _, _ = dragDoDrop.Call(dp, sp, 1, uintptr(unsafe.Pointer(&effect)))
	runtime.KeepAlive(d)
	runtime.KeepAlive(source)
	if uint32(hr) == 0 || uint32(hr) == 0x40100 || uint32(hr) == 0x40101 {
		return nil
	}
	return fmt.Errorf("拖动未完成（%08x）", uint32(hr))
}
