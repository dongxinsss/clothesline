# Contributing / 参与贡献

Thank you for improving Clothesline. Small bug reports, documentation fixes,
translations, and platform testing are welcome.

## Report a problem

Include the app version, OS version, display scaling, monitor arrangement, and
steps to reproduce. Replace personal screenshots with synthetic examples. Never
attach `shelf.json`, clipboard history, credentials, or private screen contents.

## Develop

Use Go 1.27.2 or the compatible version declared in `go.mod`.
On macOS, install Xcode Command Line Tools. Windows needs WebView2 Runtime.
The app uses the system webview and does not require npm to build.

```sh
go test -race ./...
go vet ./...
node tests/idle.test.cjs
node tests/schedule_dates.test.cjs
node tests/quit.test.cjs
```

Build instructions are in [BUILD.md](docs/BUILD.md). Use `CLOTHESLINE_DATA_DIR`
with an empty, separate directory for development so tests cannot mix with your
real clipboard history. Leave auto-collection off when creating public visuals.

## Send a change

1. Open an issue for substantial behavior changes.
2. Fork, create a focused branch, and keep platform-specific code in its native files.
3. Add meaningful regression coverage where appropriate, and update relevant guides.
4. State what was tested on real hardware and what was only compiled.
5. Open a pull request. Never claim Windows compatibility from a cross-build alone.

Documentation languages currently available: 简体中文、繁體中文、English、日本語.
These are documentation translations; the app's interface currently uses Chinese.

By contributing, you agree that your contribution may be distributed under this
repository's MIT license. Existing dependencies retain their own licenses in
[THIRD-PARTY-NOTICES.txt](docs/THIRD-PARTY-NOTICES.txt).
