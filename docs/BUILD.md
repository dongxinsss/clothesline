# Build Clothesline / 从源码构建

The application uses Go and Wails v2 with the OS webview. It has no npm build step.
Use Go **1.27.2**, as declared in `go.mod`, or a compatible newer toolchain.
Do not build inside a folder containing your private history.

## macOS

Requires Xcode Command Line Tools and macOS native build tools.

```sh
sh scripts/build-macos.sh --universal
open build/bin/拾光绳.app
```

The universal build includes arm64 and amd64. The default build creates the local
architecture only. Ad-hoc signing is used unless `CODESIGN_IDENTITY` is supplied;
this is not notarization. The app targets macOS 12.0+.

## Windows x64

Use Windows 10/11 with WebView2 Runtime installed. In PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-windows.ps1
.\build\bin\Clothesline-windows-amd64.exe
```

The build script regenerates the embedded icon resource from `frontend/appicon.ico`.
Generated `.syso` files are intentionally not stored in the repository.

Cross-build on macOS, without executing the Windows binary:

```sh
go -C scripts/winicon run . ../../frontend/appicon.ico ../../rsrc_windows_amd64.syso
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags desktop,production -ldflags '-s -w -H=windowsgui' -o build/bin/Clothesline-windows-amd64.exe .
```

A successful cross-build does not validate Windows UI or native integrations.

## Tests

```sh
go test -race ./...
go vet ./...
node tests/idle.test.cjs
node tests/schedule_dates.test.cjs
node tests/quit.test.cjs
```

On Windows, full `go vet` currently reports `unsafe.Pointer` warnings in the OLE
COM bridge (`drag_windows.go`). See [validation notes](VALIDATION.md); do not hide
this limitation when reporting results. Node is used only for these optional
JavaScript tests and does not ship with the app.

For isolated native testing, set `CLOTHESLINE_DATA_DIR` to a new empty directory,
disable clipboard collection and global shortcuts, and use only synthetic data.
The environment override also selects an independent single-instance lock.

## Browser demo

```sh
python3 -m http.server 8765 --bind 127.0.0.1 --directory frontend
```

Open `http://127.0.0.1:8765/preview.html`. The demo uses synthetic data in browser
storage; it does not provide native capture, tray integration, or native drag out.

## Layout

- `app.go`, `main.go`: app lifecycle, shelf behavior, Wails binding.
- `*_darwin.go`, `*.m`, `*_windows.go`: platform integration.
- `internal/shelf`: records, settings, history, schedules.
- `internal/longshot`: scrolling-image stitching.
- `frontend`: embedded HTML/CSS/JS and editable SVG icon sources.
- `docs/guides`, `docs/i18n`: multilingual user documentation.
- `docs/media`: public synthetic-demo screenshots and marketing artwork.

Existing icon assets are ready to build. Editing and re-exporting icons may need
Node/sharp and Pillow; those tools are not application runtime dependencies.
