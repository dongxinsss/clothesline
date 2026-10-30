package main

import (
	"clothesline/internal/shelf"
	"github.com/energye/systray"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"runtime"
)

func applyPlatformSettings(shelf.Settings) {}

func setupTray(a *App) func() {
	// Win32 windows and their message pump must live on the same OS thread.
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			ico, _ := assets.ReadFile("frontend/tray.ico")
			systray.SetIcon(ico)
			systray.SetTooltip("拾光绳 · 随手挂起一点灵感")
			systray.SetOnClick(func(systray.IMenu) { a.Expand() })
			systray.AddMenuItem("展开拾光绳", "").Click(a.Expand)
			systray.AddMenuItem("截取屏幕…", "").Click(func() {
				if e := a.Capture(); e != nil {
					a.notice(e.Error())
				}
			})
			systray.AddMenuItem("挂上剪贴板内容", "").Click(func() {
				if e := a.Paste(); e != nil {
					a.notice(e.Error())
				}
			})
			systray.AddMenuItem("新建文字便签", "").Click(func() { a.Expand(); wr.EventsEmit(a.ctx, "compose") })
			systray.AddSeparator()
			systray.AddMenuItem("退出拾光绳", "").Click(a.Quit)
		}, func() {})
	}()
	return systray.Quit
}
