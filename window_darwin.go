package main

/*
#cgo LDFLAGS: -framework Carbon
void clWindowSetup(void);
void clShowQuietly(void);
void clWorkArea(int *width, int *height);
void clMainScreenSize(int *width, int *height);
void clPlaceWindow(int x, int y, int width, int height, int aboveMenuBar);
int clSetHotkeys(int enabled);
void clSetResizable(int on);
*/
import "C"
import "sync/atomic"
import "clothesline/internal/shelf"
import wr "github.com/wailsapp/wails/v2/pkg/runtime"

func placeBasket(a *App, saved *shelf.Point) {
	w, h := workArea(a)
	p := shelf.Point{X: w - 96, Y: h - 104}
	if saved != nil {
		p = *saved
	}
	p = clampBasket(p, shelf.Frame{W: w, H: h}, 80, 80)
	wr.WindowSetPosition(a.ctx, p.X, p.Y)
}

var hotkeyLabels = [2]string{"⌃⌥A", "⌃⌥S"}
var hotkeyApp atomic.Pointer[App]

func platformWindowSetup(a *App) {
	hotkeyApp.Store(a)
	C.clWindowSetup()
}
func showWindowQuietly(*App) { C.clShowQuietly() }
func workArea(*App) (int, int) {
	var w, h C.int
	C.clWorkArea(&w, &h)
	return int(w), int(h)
}
func mainScreenSize(*App) (int, int) {
	var w, h C.int
	C.clMainScreenSize(&w, &h)
	return int(w), int(h)
}

// placeWindow uses main-screen points from the top-left corner, matching the
// coordinates of screen captures.
func placeWindow(_ *App, x, y, w, h int, aboveMenuBar bool) {
	above := 0
	if aboveMenuBar {
		above = 1
	}
	C.clPlaceWindow(C.int(x), C.int(y), C.int(w), C.int(h), C.int(above))
}

func setResizable(_ *App, on bool) {
	v := 0
	if on {
		v = 1
	}
	C.clSetResizable(C.int(v))
}

// setHotkeys returns the labels of shortcuts another app already holds.
func setHotkeys(_ *App, enabled bool) []string {
	on := 0
	if enabled {
		on = 1
	}
	failed := int(C.clSetHotkeys(C.int(on)))
	var taken []string
	for i, label := range hotkeyLabels {
		if failed&(1<<i) != 0 {
			taken = append(taken, label)
		}
	}
	return taken
}

//export clHotkeyPressed
func clHotkeyPressed(id C.int) {
	if a := hotkeyApp.Load(); a != nil {
		go a.hotkey(int(id))
	}
}
