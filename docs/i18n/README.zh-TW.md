[简体中文](../../README.md) · [繁體中文](README.zh-TW.md) · [English](README.en.md) · [日本語](README.ja.md)

# 拾光繩 · Clothesline

**把截圖掛起來，把工作接下去。**

macOS / Windows 的本機截圖、文字與日程收納工具。展開是一條桌面掛繩，收起是一個可拖動的小籃子。

[下載預覽版](https://github.com/dongxinsss/clothesline/releases) · [使用教學](../guides/guide.zh-TW.md) · [問題回報](https://github.com/dongxinsss/clothesline/issues)

![拾光繩示例畫面](../media/overview.jpg)

## 隨手收好，等會兒再用

- 截圖複製到剪貼簿，就能自動掛上；一般文字不會自動記錄。
- 手動貼上文字或圖片、拖入圖片，也能新建文字便箋。
- 將卡片拖到支援的應用程式；也可複製或匯出。目的應用程式的支援情況仍需實測。
- 歷史記錄在重新啟動後保留，移除的卡片可再次掛上。
- 日程可查看今天、本週或本月。四套主題、獨立字級與自動收起時間可調。
- 預設約佔可用畫面高度 20%；收起為 80×80 邏輯像素的小入口。

## 先試一遍

1. 從 Releases 下載對應系統的壓縮檔並解壓縮。macOS 開啟 `拾光绳.app`，Windows 執行 `Clothesline.exe`。
2. 在偏好設定確認是否開啟「自动收集截图」，複製一張不含私密資料的截圖。
3. 點「文字」旁的新建文字按鈕，寫下便箋；Esc 收起，點小籃子展開。
4. 打開「历史记录」或「日程」，試試搜尋與今天／本週／本月視圖。

## 支援狀態

macOS 12+ 提供 Apple Silicon / Intel 通用包。Windows 10/11 x64 需要 WebView2 Runtime。這是 0.4.1 預覽版：macOS 已做本機測試，Windows 完成交叉編譯，尚未真機驗證。日程沒有系統提醒或雲端同步。

**教學有四種語言；應用程式介面目前為中文。** 截圖為瀏覽器互動展示，僅含示例資料，不代表 Windows 原生功能已通過測試。

[完整教學](../guides/guide.zh-TW.md) · [建置原始碼](../BUILD.md) · [驗證記錄](../VALIDATION.md) · [MIT 授權](../../LICENSE)

如果這個小工具適合你的工作流程，歡迎 Star、分享，或提供可重現的問題回報。
