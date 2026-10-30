# Changelog

## 0.4.1 — 2026-10-28

- Remember basket position based on window mode, without requiring an exact 80×80 window-size readback.
- Restore Windows basket coordinates in the virtual desktop coordinate system with DPI-aware sizing and visible-work-area clamping.
- Keep Windows shelf/basket off the taskbar while preserving the notification-area tray.
- Allow system quit and cancel background capture/clipboard work without waiting on capture locks.
- Ensure failed or stalled settings saves cannot disable the quit button.
- Validation: automated regression tests and cross-builds; native Windows behavior still needs real-host testing.

## 0.4.0 — preview

- Native outgoing card drag, configurable idle collapse, resizable shelf, and saved window placement.
- Global capture shortcuts and automatic sampling for manual scrolling capture.
- History management and clipboard duplicate handling improvements.
- macOS capture permission handling and collapse animation updates.

## 0.3.0 — preview

- Local day/week/month schedules.
- Four appearance themes and independent font/card size settings.
- Persistent history, previous-index backup, and legacy-history import.

## 0.1.1 / 0.1.0 — initial previews

- Screenshot and text shelf, collapsible basket, clipboard image collection, local storage, and tray entry.
- App and tray icon refresh.

## Documentation publication — 2026-10-30

- Public project introduction, four-language guides (简体中文 / 繁體中文 / English / 日本語), fresh synthetic-demo screenshots, sharing copy, contribution templates, and MIT license.
- Documentation translations do not change the app's current Chinese interface.
