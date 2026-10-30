package main

import (
	"bytes"
	"context"
	"errors"
	"golang.design/x/clipboard"
	"os/exec"
	"time"
)

func captureScreen(ctx context.Context) ([]byte, error) {
	if err := clipboard.Init(); err != nil {
		return nil, err
	}
	before, _ := clipboard.Read(ctx, clipboard.FmtImage)
	watchCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	changes := clipboard.Watch(watchCtx, clipboard.FmtImage)
	if err := exec.Command("explorer.exe", "ms-screenclip:").Start(); err != nil {
		return nil, errors.New("无法打开系统截图工具，请使用 Win+Shift+S")
	}
	for {
		select {
		case <-watchCtx.Done():
			return nil, errCaptureCanceled
		case d, ok := <-changes:
			if !ok {
				return nil, errCaptureCanceled
			}
			if !d.Sensitive && len(d.Bytes) > 0 && !bytes.Equal(before, d.Bytes) {
				return d.Bytes, nil
			}
		}
	}
}

// Windows has no per-app screen capture permission for the snipping tool or GDI.
func ensureScreenCapture() error { return nil }
