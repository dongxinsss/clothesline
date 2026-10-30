package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"

	"clothesline/internal/longshot"
)

func screencaptureFile(ctx context.Context, args ...string) ([]byte, error) {
	f, err := os.CreateTemp("", "clothesline-scroll-*.png")
	if err != nil {
		return nil, err
	}
	name := f.Name()
	f.Close()
	defer os.Remove(name)
	args = append([]string{"-x", "-t", "png"}, append(args, name)...)
	if err = exec.CommandContext(ctx, "/usr/sbin/screencapture", args...).Run(); err != nil {
		return nil, errors.New("无法读取屏幕，请在系统设置 → 隐私与安全性 → 屏幕录制中允许拾光绳")
	}
	return os.ReadFile(name)
}

// captureFullScreen also returns the PNG as written, so the selection preview
// can be served without encoding the frame again.
func captureFullScreen(ctx context.Context) (image.Image, []byte, error) {
	data, err := screencaptureFile(ctx, "-m")
	if err != nil {
		return nil, nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, data, err
}

// captureRegion grabs only the selected rectangle (pt, in points), which is
// much cheaper than a whole Retina screen. px is the same area in pixels.
func captureRegion(ctx context.Context, pt, px image.Rectangle) (*image.RGBA, error) {
	data, err := screencaptureFile(ctx, "-R", fmt.Sprintf("%d,%d,%d,%d", pt.Min.X, pt.Min.Y, pt.Dx(), pt.Dy()))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if img.Bounds().Size() == px.Size() {
		return longshot.CopyRegion(img, img.Bounds())
	}
	// Mixed-scale setups can round differently; crop a full frame instead.
	full, _, err := captureFullScreen(ctx)
	if err != nil {
		return nil, err
	}
	return longshot.CopyRegion(full, px)
}
