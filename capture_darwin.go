package main

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
static int clScreenCaptureAllowed(void) { return CGPreflightScreenCaptureAccess() ? 1 : 0; }
static int clRequestScreenCapture(void) { return CGRequestScreenCaptureAccess() ? 1 : 0; }
*/
import "C"
import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

const bundleID = "app.clothesline.desktop"

var screenAccessAsked atomic.Bool

// ensureScreenCapture checks Screen Recording before the window hides. Without
// it, screencapture quietly returns only the wallpaper or nothing at all.
func ensureScreenCapture() error {
	if C.clScreenCaptureAllowed() != 0 {
		return nil
	}
	if screenAccessAsked.CompareAndSwap(false, true) {
		// A grant made for an earlier build stays switched on in System Settings
		// but no longer matches this binary. Clear only this app's own entry so
		// the system prompt can register the current build.
		exec.Command("/usr/bin/tccutil", "reset", "ScreenCapture", bundleID).Run()
		if C.clRequestScreenCapture() != 0 {
			return nil
		}
		return errors.New("需要「屏幕录制」权限：请在系统提示中允许拾光绳，按提示退出并重新打开后再截图")
	}
	exec.Command("/usr/bin/open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture").Start()
	return errors.New("屏幕录制权限尚未生效：已打开系统设置，请打开「拾光绳」的开关，然后退出并重新打开拾光绳")
}

func captureScreen(ctx context.Context) ([]byte, error) {
	f, err := os.CreateTemp("", "clothesline-capture-*.png")
	if err != nil {
		return nil, err
	}
	p := f.Name()
	f.Close()
	defer os.Remove(p)
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-i", "-x", "-t", "png", p)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.TrimSpace(string(output)) == "" {
			return nil, errCaptureCanceled
		}
		return nil, errors.New("截图未完成，请在系统设置 → 隐私与安全性 → 屏幕录制中检查拾光绳的权限")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, errCaptureCanceled
	}
	return b, nil
}
