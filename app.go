package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"clothesline/internal/longshot"
	"clothesline/internal/shelf"
	"github.com/wailsapp/wails/v2/pkg/options"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/clipboard"
)

type App struct {
	ctx           context.Context
	workCtx       context.Context
	cancelWork    context.CancelFunc
	store         *shelf.Store
	mu            sync.Mutex
	watchCancel   context.CancelFunc
	trayEnd       func()
	quit          bool
	collapsed     bool
	clipboardOK   bool
	capturing     bool
	dragging      bool
	panelOpen     bool
	shelfLaid     shelf.Frame // the shelf frame layout last set
	undo          *shelf.Item
	memoryTimer   *time.Timer
	scrollMu      sync.Mutex
	scrollSession *longshot.Session
	scrollPreview image.Image
	scrollPNG     []byte
	scrollRect    image.Rectangle // selected region in screen pixels
	scrollPt      image.Rectangle // the same region in points
	scrollScreen  image.Point
	scrollAuto    bool
	scrollCancel  context.CancelFunc
	// selfCopy holds the hash of the last image this app put on the clipboard,
	// so auto-collect does not hang it back up (or restore it from history).
	selfCopy atomic.Value
}

var errCaptureCanceled = errors.New("截图已取消")

func newApp(s *shelf.Store) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{store: s, workCtx: ctx, cancelWork: cancel}
}
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.cancelWork()
	a.workCtx, a.cancelWork = context.WithCancel(ctx)
	a.clipboardOK = clipboard.Init() == nil
}
func (a *App) ready(ctx context.Context) {
	platformWindowSetup(a)
	a.layout(false, true)
	a.trayEnd = setupTray(a)
	applyPlatformSettings(a.store.Snapshot().Settings)
	a.applyHotkeys()
	a.restartWatcher()
	go a.cleanDragExports()
	if !a.clipboardOK {
		a.notice("剪贴板暂不可用，仍可导入图片和新建便签")
	}
}
func (a *App) shutdown(context.Context) {
	a.prepareQuit()
	if a.trayEnd != nil {
		a.trayEnd()
	}
}

// Never take scrollMu here: a capture can hold it while waiting for the OS.
func (a *App) prepareQuit() {
	a.mu.Lock()
	if a.quit {
		a.mu.Unlock()
		return
	}
	a.quit = true
	cancel, watch := a.cancelWork, a.watchCancel
	if a.memoryTimer != nil {
		a.memoryTimer.Stop()
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if watch != nil {
		watch()
	}
}
func (a *App) beforeClose(context.Context) bool {
	a.rememberBasket()
	a.prepareQuit()
	return false
}
func (a *App) isQuitting() bool                          { a.mu.Lock(); defer a.mu.Unlock(); return a.quit }
func (a *App) secondInstance(options.SecondInstanceData) { a.Expand() }
func (a *App) notice(s string) {
	if !a.isQuitting() {
		wr.EventsEmit(a.ctx, "notice", s)
	}
}
func (a *App) changed() {
	if a.isQuitting() {
		return
	}
	// History can grow without bound; the panel fetches it when shown.
	wr.EventsEmit(a.ctx, "changed", a.store.Summary())
	// Large image buffers should not remain resident throughout a workday.
	// Debounce reclamation so bursts of screenshots can reuse memory.
	a.mu.Lock()
	if a.memoryTimer != nil {
		a.memoryTimer.Stop()
	}
	a.memoryTimer = time.AfterFunc(5*time.Second, debug.FreeOSMemory)
	a.mu.Unlock()
}
func (a *App) GetState() shelf.State    { return a.store.Snapshot() }
func (a *App) GetHistory() []shelf.Item { return a.store.History() }
func (a *App) Platform() string         { return runtime.GOOS }
func (a *App) SaveSchedule(id string, d shelf.ScheduleDraft) error {
	_, err := a.store.SaveSchedule(id, d)
	if err == nil {
		a.changed()
	}
	return err
}
func (a *App) SetScheduleDone(id string, done bool) error {
	err := a.store.SetScheduleDone(id, done)
	if err == nil {
		a.changed()
	}
	return err
}

// layout sizes the window inside the work area of its screen (without menu
// bar, Dock or taskbar). Only an expansion the user asked for activates the
// app; a collapse must not take focus from what they are typing in.
func (a *App) layout(collapsed, activate bool) {
	if a.isQuitting() {
		return
	}
	w, h := workArea(a)
	if collapsed {
		setResizable(a, false)
		wr.WindowSetMinSize(a.ctx, 68, 68)
		wr.WindowSetSize(a.ctx, 80, 80)
		placeBasket(a, a.store.Snapshot().Basket)
	} else {
		f := shelfFrame(w, h, a.store.Snapshot())
		wr.WindowSetSize(a.ctx, f.W, f.H)
		wr.WindowSetPosition(a.ctx, f.X, f.Y)
		wr.WindowSetMinSize(a.ctx, min(shelfMinWidth, w), min(shelfMinHeight, h))
		setResizable(a, true)
		a.mu.Lock()
		a.shelfLaid = f
		a.mu.Unlock()
	}
	if activate {
		wr.WindowShow(a.ctx)
	} else {
		showWindowQuietly(a)
	}
}

const shelfMinWidth, shelfMinHeight = 360, 140

// shelfFrame is the frame the user last gave the shelf, kept inside the work
// area, or the default strip along the top of the screen.
func shelfFrame(w, h int, s shelf.State) shelf.Frame {
	if f := s.Shelf; f != nil {
		fw := max(min(f.W, w), min(shelfMinWidth, w))
		fh := max(min(f.H, h), min(shelfMinHeight, h))
		return shelf.Frame{X: max(0, min(f.X, w-fw)), Y: max(0, min(f.Y, h-fh)), W: fw, H: fh}
	}
	width := min(w-32, 1280)
	height := max(160+max(0, s.Settings.FontSize-14)*4, h*s.Settings.HeightPercent/100)
	return shelf.Frame{X: (w - width) / 2, Y: 10, W: width, H: height}
}

// rememberShelf keeps a size or position the user gave the shelf. It runs
// before the window turns into something else (basket, panel, capture).
func (a *App) rememberShelf() {
	a.mu.Lock()
	skip, laid := a.collapsed || a.panelOpen || a.capturing, a.shelfLaid
	a.mu.Unlock()
	if skip {
		return
	}
	x, y := wr.WindowGetPosition(a.ctx)
	w, h := wr.WindowGetSize(a.ctx)
	f := shelf.Frame{X: x, Y: y, W: w, H: h}
	if f == laid {
		return
	}
	if err := a.store.SetShelf(&f); err != nil {
		a.notice(err.Error())
		return
	}
	a.mu.Lock()
	a.shelfLaid = f
	a.mu.Unlock()
}
func (a *App) Collapse() {
	if a.isQuitting() {
		return
	}
	a.scrollMu.Lock()
	scrollActive := a.scrollSession != nil || a.scrollPreview != nil
	a.scrollMu.Unlock()
	if scrollActive {
		a.CancelScrollCapture()
		return
	}
	a.rememberShelf()
	a.rememberBasket()
	a.mu.Lock()
	wasCollapsed := a.collapsed
	a.collapsed = true
	a.mu.Unlock()
	change := func() {
		wr.EventsEmit(a.ctx, "mode", "basket")
		a.layout(true, false)
	}
	if wasCollapsed {
		change()
		return
	}
	animateWindow(false, change)
}
func (a *App) Expand() {
	if a.isQuitting() {
		return
	}
	a.rememberBasket()
	a.mu.Lock()
	wasCollapsed := a.collapsed
	a.collapsed = false
	a.mu.Unlock()
	change := func() {
		a.layout(false, true)
		wr.EventsEmit(a.ctx, "mode", "shelf")
	}
	if !wasCollapsed {
		change()
		return
	}
	animateWindow(true, change)
}

// rememberBasket keeps where the user dragged the basket for the next collapse
// and the next launch.
func (a *App) rememberBasket() {
	a.rememberBasketPosition(func() (int, int) { return wr.WindowGetPosition(a.ctx) })
}

func (a *App) rememberBasketPosition(position func() (int, int)) {
	a.mu.Lock()
	visible := a.collapsed && !a.capturing && !a.panelOpen && !a.quit
	a.mu.Unlock()
	if !visible {
		return
	}
	x, y := position()
	if err := a.store.SetBasket(shelf.Point{X: x, Y: y}); err != nil {
		a.notice(err.Error())
	}
}

// reveal brings the window back after a capture hid it, in the state it was
// in and without taking focus from the app that was just captured.
func (a *App) reveal() {
	if a.isQuitting() {
		return
	}
	a.mu.Lock()
	collapsed := a.collapsed
	a.mu.Unlock()
	if collapsed {
		wr.EventsEmit(a.ctx, "mode", "basket")
		a.layout(true, false)
		return
	}
	a.layout(false, false)
	wr.EventsEmit(a.ctx, "mode", "shelf")
}
func (a *App) SetPanel(open bool) {
	if open {
		a.rememberShelf()
	}
	a.mu.Lock()
	a.panelOpen = open
	a.mu.Unlock()
	if open {
		setResizable(a, false)
		w, h := workArea(a)
		wr.WindowSetSize(a.ctx, min(900, w-32), min(560, h-40))
		wr.WindowCenter(a.ctx)
	} else {
		a.layout(false, true)
	}
}
func (a *App) AutoCollapse() bool {
	a.mu.Lock()
	blocked := a.capturing || a.dragging || a.panelOpen || a.quit || a.collapsed
	a.mu.Unlock()
	if blocked {
		return false
	}
	a.Collapse()
	return true
}
func (a *App) DragItem(id string) error {
	i, err := a.store.Find(id)
	if err != nil {
		return err
	}
	path, text := "", i.Text
	var thumb []byte
	if i.Kind == "image" {
		path, err = a.store.ImagePath(id, false)
		if err != nil {
			return err
		}
		exports := filepath.Join(a.store.Dir(), "DragExports")
		if err = os.MkdirAll(exports, 0700); err != nil {
			return err
		}
		friendly := filepath.Join(exports, "拾光-"+i.CreatedAt.Local().Format("20060102-150405")+"-"+i.ID[:8]+".png")
		if _, e := os.Stat(friendly); errors.Is(e, os.ErrNotExist) {
			if e = os.Link(path, friendly); e != nil {
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				if e = os.WriteFile(friendly, data, 0600); e != nil {
					return e
				}
			}
		}
		path = friendly
		if p, e := a.store.ImagePath(id, true); e == nil {
			thumb, _ = os.ReadFile(p)
		}
	}
	a.mu.Lock()
	if a.dragging || a.capturing {
		a.mu.Unlock()
		return errors.New("请先完成当前拖动或截图")
	}
	a.dragging = true
	a.mu.Unlock()
	err = startNativeDrag(path, text, thumb)
	if err != nil || runtime.GOOS == "windows" {
		a.dragEnded()
	}
	return err
}
func (a *App) dragEnded() {
	a.mu.Lock()
	a.dragging = false
	a.mu.Unlock()
	wr.EventsEmit(a.ctx, "drag-ended")
}
func (a *App) screenWidth() int {
	ss, e := wr.ScreenGetAll(a.ctx)
	if e == nil {
		for _, s := range ss {
			if s.IsPrimary {
				return s.Size.Width
			}
		}
	}
	return 1440
}
func (a *App) screenHeight() int {
	ss, e := wr.ScreenGetAll(a.ctx)
	if e == nil {
		for _, s := range ss {
			if s.IsPrimary {
				return s.Size.Height
			}
		}
	}
	return 900
}
func (a *App) SaveSettings(v shelf.Settings) error {
	a.rememberBasket()
	previous := a.store.Snapshot().Settings
	if err := a.store.UpdateSettings(v); err != nil {
		return err
	}
	if previous.AlwaysOnTop != v.AlwaysOnTop {
		wr.WindowSetAlwaysOnTop(a.ctx, v.AlwaysOnTop)
	}
	if previous.ShowDockIcon != v.ShowDockIcon {
		applyPlatformSettings(v)
	}
	if previous.AutoCollect != v.AutoCollect {
		a.restartWatcher()
	}
	if previous.DisableHotkeys != v.DisableHotkeys {
		a.applyHotkeys()
	}
	if previous.HeightPercent != v.HeightPercent {
		// Picking a height goes back to the default strip.
		if err := a.store.SetShelf(nil); err != nil {
			return err
		}
	}
	a.changed()
	a.mu.Lock()
	collapsed := a.collapsed
	a.mu.Unlock()
	if collapsed {
		a.layout(true, false)
	}
	return nil
}

func (a *App) RestoreItem(id string) error {
	i, err := a.store.Find(id)
	if err != nil {
		return err
	}
	if err = a.store.Restore(i); err != nil {
		return err
	}
	a.changed()
	return nil
}
func (a *App) ImportHistory() error {
	dir, err := wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{Title: "选择包含 shelf.json 的旧记录目录"})
	if err != nil || dir == "" {
		return err
	}
	n, err := a.store.MergeFrom(dir)
	if err != nil {
		return err
	}
	a.changed()
	a.notice(fmt.Sprintf("已找回 %d 条旧记录；原目录未改动", n))
	return nil
}
func (a *App) restartWatcher() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.watchCancel != nil {
		a.watchCancel()
		a.watchCancel = nil
	}
	if a.quit || !a.clipboardOK || !a.store.Snapshot().Settings.AutoCollect {
		return
	}
	ctx, cancel := context.WithCancel(a.workCtx)
	a.watchCancel = cancel
	go func() {
		for d := range clipboard.Watch(ctx, clipboard.FmtImage) {
			if ctx.Err() != nil {
				return
			}
			if d.Sensitive || len(d.Bytes) == 0 || a.selfCopy.Load() == sha256.Sum256(d.Bytes) {
				continue
			}
			_, added, e := a.store.AddImage(d.Bytes, "剪贴板")
			if e != nil {
				a.notice(e.Error())
			} else if added {
				a.changed()
			}
		}
	}()
}
func (a *App) AddText(text string) error {
	_, added, e := a.store.AddText(text, "便签")
	if e == nil {
		a.changed()
		if !added {
			a.notice("这段文字已经挂在绳上了")
		}
	}
	return e
}
func (a *App) AddImage(encoded string) error {
	if len(encoded) > 46<<20 {
		return errors.New("图片不能超过 32 MB")
	}
	data, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return e
	}
	_, added, e := a.store.AddImage(data, "导入")
	if e == nil {
		a.changed()
		if !added {
			a.notice("这张图片已经挂在绳上了")
		}
	}
	return e
}
func (a *App) Paste() error {
	if !a.clipboardOK {
		return errors.New("系统剪贴板不可用")
	}
	if sensitive, _ := clipboard.Sensitive(a.ctx); sensitive {
		return errors.New("此内容被来源应用标记为敏感，已跳过")
	}
	format, data, e := readClipboardContent(func(f clipboard.Format) ([]byte, error) { return clipboard.Read(a.ctx, f) })
	if e != nil {
		return e
	}
	if format == clipboard.FmtImage {
		_, added, e := a.store.AddImage(data, "剪贴板")
		if e == nil {
			a.changed()
			if !added {
				a.notice("这张图片已经挂在绳上了")
			}
		}
		return e
	}
	_, added, e := a.store.AddText(string(data), "剪贴板")
	if e == nil {
		a.changed()
		if !added {
			a.notice("这段文字已经挂在绳上了")
		}
	}
	return e
}
func (a *App) CopyItem(id string) error {
	i, e := a.store.Find(id)
	if e != nil {
		return e
	}
	if !a.clipboardOK {
		return errors.New("系统剪贴板不可用")
	}
	if i.Kind == "text" {
		_, e = clipboard.Write(a.ctx, clipboard.FmtText, []byte(i.Text))
		return e
	}
	p, e := a.store.ImagePath(id, false)
	if e != nil {
		return e
	}
	data, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	a.selfCopy.Store(sha256.Sum256(data))
	_, e = clipboard.Write(a.ctx, clipboard.FmtImage, data)
	return e
}
func (a *App) RemoveItem(id string) error {
	i, e := a.store.Find(id)
	if e != nil {
		return e
	}
	if e = a.store.Remove(id); e != nil {
		return e
	}
	a.mu.Lock()
	a.undo = &i
	a.mu.Unlock()
	a.changed()
	return nil
}
func (a *App) UndoRemove() error {
	a.mu.Lock()
	i := a.undo
	a.mu.Unlock()
	if i == nil {
		return nil
	}
	if err := a.store.Restore(*i); err != nil {
		return err
	}
	a.mu.Lock()
	a.undo = nil
	a.mu.Unlock()
	a.changed()
	return nil
}
func (a *App) ImportImages() error {
	files, e := wr.OpenMultipleFilesDialog(a.ctx, wr.OpenDialogOptions{Title: "把图片挂上拾光绳", Filters: []wr.FileFilter{{DisplayName: "图片", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp"}}})
	if e != nil {
		return e
	}
	for _, p := range files {
		if e = a.importFile(p); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) importFile(p string) error {
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() > shelf.MaxImageBytes {
		return errors.New("请选择小于 32 MB 的图片文件")
	}
	data, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	_, _, e = a.store.AddImage(data, "导入")
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) ImportPaths(paths []string) error {
	for _, p := range paths {
		if e := a.importFile(p); e != nil {
			return fmt.Errorf("%s：%w", filepath.Base(p), e)
		}
	}
	return nil
}
func (a *App) ExportItem(id string) error {
	i, e := a.store.Find(id)
	if e != nil {
		return e
	}
	ext := ".png"
	if i.Kind == "text" {
		ext = ".txt"
	}
	p, e := wr.SaveFileDialog(a.ctx, wr.SaveDialogOptions{Title: "保存到文件", DefaultFilename: "拾光-" + i.CreatedAt.Format("20060102-150405") + ext})
	if e != nil || p == "" {
		return e
	}
	if i.Kind == "text" {
		return os.WriteFile(p, []byte(i.Text), 0600)
	}
	src, e := a.store.ImagePath(id, false)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(src)
	if e != nil {
		return e
	}
	return os.WriteFile(p, b, 0600)
}
func (a *App) Capture() error {
	a.rememberBasket()
	a.rememberShelf()
	a.mu.Lock()
	if a.capturing || a.quit {
		a.mu.Unlock()
		return errors.New("截图工具已经打开")
	}
	a.capturing = true
	a.mu.Unlock()
	if err := ensureScreenCapture(); err != nil {
		a.mu.Lock()
		a.capturing = false
		a.mu.Unlock()
		return err
	}
	go func() {
		defer func() { a.mu.Lock(); a.capturing = false; a.mu.Unlock() }()
		wr.WindowHide(a.ctx)
		time.Sleep(220 * time.Millisecond)
		data, e := captureScreen(a.workCtx)
		if a.isQuitting() {
			return
		}
		a.reveal()
		if e != nil {
			if !errors.Is(e, errCaptureCanceled) {
				a.notice(e.Error())
			}
			return
		}
		if len(data) > 0 {
			_, _, e = a.store.AddImage(data, "截图")
			if e != nil {
				a.notice(e.Error())
			} else {
				a.changed()
			}
		}
	}()
	return nil
}
func (a *App) Quit() {
	a.rememberBasket()
	a.rememberShelf()
	a.prepareQuit()
	wr.Quit(a.ctx)
}

// hotkey handles the global shortcuts: 1 captures a region, 2 starts a
// scrolling capture.
func (a *App) hotkey(id int) {
	var err error
	switch id {
	case 1:
		err = a.Capture()
	case 2:
		err = a.StartScrollCapture()
	}
	if err != nil {
		a.notice(err.Error())
	}
}
func (a *App) applyHotkeys() {
	if taken := setHotkeys(a, !a.store.Snapshot().Settings.DisableHotkeys); len(taken) > 0 {
		a.notice("快捷键 " + strings.Join(taken, "、") + " 已被其他应用占用，可在偏好设置中关闭全局快捷键")
	}
}
func (a *App) HotkeyLabels() [2]string { return hotkeyLabels }

// PurgeHistory permanently deletes archived records; no ids means all of them.
func (a *App) PurgeHistory(ids []string) error {
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	gone := map[string]bool{}
	for _, i := range a.store.History() {
		if len(ids) == 0 || wanted[i.ID] {
			gone[i.ID] = true
		}
	}
	ids = ids[:0]
	for id := range gone {
		ids = append(ids, id)
	}
	n, err := a.store.Purge(ids)
	if err != nil || n == 0 {
		return err
	}
	a.mu.Lock()
	if a.undo != nil && gone[a.undo.ID] {
		a.undo = nil
	}
	a.mu.Unlock()
	a.removeDragExports(func(name string) bool {
		for id := range gone {
			if strings.Contains(name, "-"+id[:8]+".png") {
				return true
			}
		}
		return false
	})
	a.changed()
	a.notice(fmt.Sprintf("已永久删除 %d 条记录", n))
	return nil
}

// Dragged images are exported as files other apps copy on drop; old ones are
// only clutter.
func (a *App) cleanDragExports() {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	a.removeDragExports(func(name string) bool {
		info, err := os.Stat(filepath.Join(a.store.Dir(), "DragExports", name))
		return err == nil && info.ModTime().Before(cutoff)
	})
}
func (a *App) removeDragExports(match func(name string) bool) {
	dir := filepath.Join(a.store.Dir(), "DragExports")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Type().IsRegular() && match(e.Name()) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
