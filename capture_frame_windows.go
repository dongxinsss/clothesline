package main

import (
	"context"
	"errors"
	"image"

	"clothesline/internal/longshot"
	"github.com/kbinani/screenshot"
)

func captureFullScreen(ctx context.Context) (image.Image, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, nil, errors.New("没有可用的屏幕")
	}
	index := 0
	for i := 0; i < n; i++ {
		if screenshot.GetDisplayBounds(i).Min == (image.Point{}) {
			index = i
			break
		}
	}
	img, err := screenshot.CaptureRect(screenshot.GetDisplayBounds(index))
	return img, nil, err
}
func captureRegion(ctx context.Context, _, px image.Rectangle) (*image.RGBA, error) {
	full, _, err := captureFullScreen(ctx)
	if err != nil {
		return nil, err
	}
	return longshot.CopyRegion(full, px)
}
