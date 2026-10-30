package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"math"
	"net/http"
	"time"

	"clothesline/internal/longshot"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) StartScrollCapture() error {
	a.rememberBasket()
	a.scrollMu.Lock()
	defer a.scrollMu.Unlock()
	a.rememberShelf()
	a.mu.Lock()
	if a.capturing || a.quit {
		a.mu.Unlock()
		return errors.New("请先完成或取消当前截图")
	}
	a.capturing = true
	a.mu.Unlock()
	if err := ensureScreenCapture(); err != nil {
		a.resetScrollLocked()
		return err
	}
	wr.WindowHide(a.ctx)
	time.Sleep(220 * time.Millisecond)
	frame, encoded, err := captureFullScreen(a.workCtx)
	if a.isQuitting() {
		return context.Canceled
	}
	if err == nil && int64(frame.Bounds().Dx())*int64(frame.Bounds().Dy()) > 50_000_000 {
		err = errors.New("当前屏幕分辨率过大")
	}
	if err == nil && encoded == nil {
		var b bytes.Buffer
		err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&b, frame)
		encoded = b.Bytes()
	}
	if err != nil {
		a.resetScrollLocked()
		a.reveal()
		return err
	}
	a.scrollPreview = frame
	a.scrollPNG = encoded
	a.scrollScreen = frame.Bounds().Size()
	wr.EventsEmit(a.ctx, "mode", "scroll-select")
	w, h := mainScreenSize(a)
	setResizable(a, false)
	placeWindow(a, 0, 0, w, h, true)
	wr.WindowShow(a.ctx)
	return nil
}
func (a *App) ConfirmScrollRegion(x, y, w, h float64) (longshot.Status, error) {
	a.scrollMu.Lock()
	defer a.scrollMu.Unlock()
	for _, v := range []float64{x, y, w, h} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return longshot.Status{}, errors.New("区域坐标无效")
		}
	}
	if a.scrollPreview == nil || x < 0 || y < 0 || w <= 0 || h <= 0 || x+w > 1.00001 || y+h > 1.00001 {
		return longshot.Status{}, errors.New("请在当前屏幕内选择区域")
	}
	size := a.scrollScreen
	sw, sh := mainScreenSize(a)
	scale := float64(size.X) / float64(sw)
	// Snap to whole points so later region captures come back the same size.
	pt := image.Rect(int(math.Ceil(x*float64(sw))), int(math.Ceil(y*float64(sh))), min(sw, int(math.Floor((x+w)*float64(sw)))), min(sh, int(math.Floor((y+h)*float64(sh)))))
	px := func(v int) int { return int(float64(v)*scale + .5) }
	rect := image.Rect(px(pt.Min.X), px(pt.Min.Y), min(size.X, px(pt.Max.X)), min(size.Y, px(pt.Max.Y)))
	tile, err := longshot.CopyRegion(a.scrollPreview, rect)
	if err != nil {
		return longshot.Status{}, err
	}
	session, err := longshot.New(tile)
	if err != nil {
		return longshot.Status{}, err
	}
	a.scrollSession = session
	a.scrollPt, a.scrollRect = pt, rect
	a.scrollPreview = nil
	a.scrollPNG = nil
	a.scrollAuto = a.showScrollControls()
	if a.scrollAuto {
		ctx, cancel := context.WithCancel(a.workCtx)
		a.scrollCancel = cancel
		go a.autoScroll(ctx)
	}
	return a.scrollStatusLocked(), nil
}

// showScrollControls puts the console beside the selected region so it stays
// visible without ending up in the captures. It reports false when there is no
// room, in which case the console hides for each manual capture instead.
func (a *App) showScrollControls() bool {
	if a.isQuitting() {
		return false
	}
	wr.EventsEmit(a.ctx, "mode", "scroll-controls")
	cw, ch := 410, 250
	if a.store.Snapshot().Settings.FontSize >= 18 {
		cw, ch = 460, 300
	}
	sw, sh := mainScreenSize(a)
	r, gap := a.scrollPt, 16
	for _, p := range []image.Point{
		{r.Max.X + gap, r.Max.Y - ch},
		{r.Min.X - gap - cw, r.Max.Y - ch},
		{r.Max.X - cw, r.Max.Y + gap},
		{r.Max.X - cw, r.Min.Y - gap - ch},
	} {
		// Keep clear of the menu bar and the screen edges.
		p.X = max(8, min(p.X, sw-cw-8))
		p.Y = max(40, min(p.Y, sh-ch-8))
		if !image.Rect(p.X, p.Y, p.X+cw, p.Y+ch).Overlaps(r) {
			placeWindow(a, p.X, p.Y, cw, ch, false)
			wr.WindowSetAlwaysOnTop(a.ctx, true)
			wr.WindowShow(a.ctx)
			return true
		}
	}
	placeWindow(a, max(0, sw-cw-22), max(40, sh-ch-80), cw, ch, false)
	wr.WindowSetAlwaysOnTop(a.ctx, true)
	wr.WindowShow(a.ctx)
	return false
}
func (a *App) scrollStatusLocked() longshot.Status {
	s := a.scrollSession.Status()
	s.Auto = a.scrollAuto
	return s
}

// autoScroll samples the region while the user scrolls the target page and
// stitches whatever moved far enough.
func (a *App) autoScroll(ctx context.Context) {
	tick := time.NewTicker(350 * time.Millisecond)
	defer tick.Stop()
	lastHint := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		a.scrollMu.Lock()
		if ctx.Err() != nil || a.scrollSession == nil {
			a.scrollMu.Unlock()
			return
		}
		grew := false
		frame, err := captureRegion(ctx, a.scrollPt, a.scrollRect)
		if err == nil {
			grew, err = a.scrollSession.Offer(frame)
		}
		status := a.scrollStatusLocked()
		if err != nil && ctx.Err() == nil {
			status.Hint = err.Error()
		}
		a.scrollMu.Unlock()
		if grew || status.Hint != lastHint {
			wr.EventsEmit(a.ctx, "scroll-status", status)
		}
		lastHint = status.Hint
	}
}
func (a *App) AddScrollSegment(overlapPercent int) (longshot.Status, error) {
	a.scrollMu.Lock()
	defer a.scrollMu.Unlock()
	if a.scrollSession == nil {
		return longshot.Status{}, errors.New("请先选择滚动截图区域")
	}
	if !a.scrollAuto {
		wr.WindowHide(a.ctx)
		defer a.showScrollControls()
		time.Sleep(220 * time.Millisecond)
	}
	frame, err := captureRegion(a.workCtx, a.scrollPt, a.scrollRect)
	if err != nil {
		return a.scrollStatusLocked(), err
	}
	if _, err = a.scrollSession.Add(frame, overlapPercent); err != nil {
		return a.scrollStatusLocked(), err
	}
	return a.scrollStatusLocked(), nil
}
func (a *App) FinishScrollCapture() error {
	a.scrollMu.Lock()
	defer a.scrollMu.Unlock()
	if a.scrollSession == nil {
		return errors.New("没有正在采集的长图")
	}
	if a.scrollCancel != nil {
		a.scrollCancel()
	}
	if err := a.scrollSession.Flush(); err != nil {
		return err
	}
	var b bytes.Buffer
	if err := png.Encode(&b, a.scrollSession.Image()); err != nil {
		return err
	}
	if _, _, err := a.store.AddImage(b.Bytes(), "滚动截图"); err != nil {
		return err
	}
	a.resetScrollLocked()
	a.reveal()
	a.changed()
	return nil
}
func (a *App) resetScrollLocked() {
	if a.scrollCancel != nil {
		a.scrollCancel()
		a.scrollCancel = nil
	}
	a.scrollSession = nil
	a.scrollPreview = nil
	a.scrollPNG = nil
	a.scrollAuto = false
	a.mu.Lock()
	a.capturing = false
	a.mu.Unlock()
	if !a.isQuitting() {
		wr.WindowSetAlwaysOnTop(a.ctx, a.store.Snapshot().Settings.AlwaysOnTop)
	}
}
func (a *App) CancelScrollCapture() {
	a.scrollMu.Lock()
	a.resetScrollLocked()
	a.scrollMu.Unlock()
	a.reveal()
}
func (a *App) serveScrollPreview(w http.ResponseWriter, r *http.Request) {
	a.scrollMu.Lock()
	data := a.scrollPNG
	a.scrollMu.Unlock()
	if len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}
