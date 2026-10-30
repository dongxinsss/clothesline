package main

import (
	"clothesline/internal/shelf"
	"context"
	"testing"
	"time"
)

func TestBasketPositionAcrossMonitorLayouts(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    shelf.Point
		area shelf.Frame
		size int
		want shelf.Point
	}{
		{"user position", shelf.Point{X: 150, Y: 350}, shelf.Frame{W: 1920, H: 1040}, 80, shelf.Point{X: 150, Y: 350}},
		{"150 percent DPI", shelf.Point{X: 150, Y: 350}, shelf.Frame{W: 1920, H: 1040}, 120, shelf.Point{X: 150, Y: 350}},
		{"left taskbar", shelf.Point{X: 0, Y: 350}, shelf.Frame{X: 48, W: 1872, H: 1080}, 80, shelf.Point{X: 48, Y: 350}},
		{"left monitor", shelf.Point{X: -1500, Y: 350}, shelf.Frame{X: -1920, W: 1920, H: 1040}, 80, shelf.Point{X: -1500, Y: 350}},
		{"unplugged monitor", shelf.Point{X: 2400, Y: 1800}, shelf.Frame{W: 1920, H: 1040}, 120, shelf.Point{X: 1800, Y: 920}},
		{"top taskbar", shelf.Point{X: 150, Y: 0}, shelf.Frame{Y: 48, W: 1920, H: 1032}, 80, shelf.Point{X: 150, Y: 48}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampBasket(tc.p, tc.area, tc.size, tc.size); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSystemQuitDoesNotCollapseOrWaitForCapture(t *testing.T) {
	a := newApp(nil)
	watch, cancel := context.WithCancel(context.Background())
	a.watchCancel = cancel
	a.scrollMu.Lock()
	defer a.scrollMu.Unlock()
	done := make(chan bool, 1)
	go func() { done <- a.beforeClose(context.Background()) }()
	select {
	case veto := <-done:
		if veto {
			t.Fatal("system quit was vetoed")
		}
	case <-time.After(time.Second):
		t.Fatal("quit blocked on capture")
	}
	if !a.isQuitting() || a.workCtx.Err() == nil || watch.Err() == nil {
		t.Fatal("background work was not canceled")
	}
	a.prepareQuit() // Repeated framework callbacks must remain safe.
}

func TestRememberBasketOnlyReadsBasketGeometry(t *testing.T) {
	store, err := shelf.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(store)
	a.collapsed = true
	p := shelf.Point{X: -1200, Y: 260}
	read := func() (int, int) { return p.X, p.Y }
	a.rememberBasketPosition(read)
	// Overlay and shelf positions must never overwrite the user's basket.
	for _, mode := range []string{"capture", "panel", "shelf", "quit"} {
		a.capturing, a.panelOpen, a.collapsed, a.quit = mode == "capture", mode == "panel", mode != "shelf", mode == "quit"
		a.rememberBasketPosition(func() (int, int) { t.Fatal("read non-basket geometry in " + mode); return 0, 0 })
	}
	reopened, err := shelf.Open(store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Snapshot().Basket; got == nil || *got != p {
		t.Fatalf("basket position lost: %v", got)
	}
}
