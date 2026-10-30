package main

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"clothesline/internal/shelf"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	debug.SetMemoryLimit(64 << 20)
	debug.SetGCPercent(75)
	root, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	if override := os.Getenv("CLOTHESLINE_DATA_DIR"); override != "" {
		root = override
	} else {
		root = filepath.Join(root, "Clothesline")
	}
	store, err := shelf.Open(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	app := newApp(store)
	instanceID := "76cb9c55-5d76-44e8-8c61-740dfe905e11"
	if os.Getenv("CLOTHESLINE_DATA_DIR") != "" {
		sum := sha256.Sum256([]byte(root))
		instanceID += fmt.Sprintf("-%x", sum[:6])
	}
	web, _ := fs.Sub(assets, "frontend")
	err = wails.Run(&options.App{
		Title: "拾光绳", Width: 1000, Height: 200, MinWidth: 68, MinHeight: 68,
		Frameless: true, AlwaysOnTop: store.Snapshot().Settings.AlwaysOnTop,
		StartHidden:      runtime.GOOS == "windows",
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &assetserver.Options{Assets: web, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/capture/background.png" {
				app.serveScrollPreview(w, r)
				return
			}
			if !strings.HasPrefix(r.URL.Path, "/media/") {
				http.NotFound(w, r)
				return
			}
			name := strings.TrimPrefix(r.URL.Path, "/media/")
			thumb := strings.HasSuffix(name, ".jpg")
			id := strings.TrimSuffix(strings.TrimSuffix(name, ".png"), ".jpg")
			p, e := store.ImagePath(id, thumb)
			if e != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "private, max-age=86400")
			http.ServeFile(w, r, p)
		})},
		OnStartup: app.startup, OnDomReady: app.ready, OnShutdown: app.shutdown,
		OnBeforeClose: app.beforeClose, Bind: []interface{}{app},
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: instanceID, OnSecondInstanceLaunch: app.secondInstance},
		DragAndDrop:        &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		Mac:                &mac.Options{WebviewIsTransparent: true, WindowIsTranslucent: false, TitleBar: &mac.TitleBar{TitlebarAppearsTransparent: true, HideTitle: true, HideTitleBar: true, FullSizeContent: true}},
		Windows:            &windows.Options{WebviewIsTransparent: true, WindowIsTranslucent: true, DisableFramelessWindowDecorations: true},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
