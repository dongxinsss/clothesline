package main

/*
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework QuartzCore
int clMinimizePrepare(void);
void clMinimizeRun(int expand);
*/
import "C"

// animateWindow runs change (the shelf/basket layout switch) behind the
// Dock-style minimize animation. Without a visible window it just switches.
func animateWindow(expand bool, change func()) {
	if C.clMinimizePrepare() == 0 {
		change()
		return
	}
	change()
	direction := 0
	if expand {
		direction = 1
	}
	C.clMinimizeRun(C.int(direction))
}
