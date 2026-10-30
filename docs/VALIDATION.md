# 拾光绳 0.1.0 验证记录

日期：2026-10-26。开发目录：`/Users/aa/Projects/clothesline`。未修改 WindowsMigration 恢复树或 WLAN 项目。

## 构建与自动验证

- Go 1.27.2，macOS 26.5.1 / arm64。
- `go test -race ./...` 通过。包含本地存储测试和四种剪贴板读取分支回归测试。
- `go vet ./...` 通过。
- `node --check frontend/app.js` 通过。
- macOS arm64 与 amd64 分别构建成功，`lipo` 合并为通用可执行文件。
- macOS 应用采用临时本地签名，可用 `codesign --verify --deep --strict` 校验。未做 Apple 开发者签名 / 公证。
- Windows amd64 / CGO_DISABLED 交叉构建成功；`file` 确认为 PE32+ GUI x86-64。**未运行 Windows 二进制，未做 Windows 真机测试**。
- Wails 的 macOS 原生代码存在上游 `setShowsBaselineSeparator` 弃用警告，不影响本次链接与运行。

## macOS 实机交互

在 `build/test-data` 隔离目录使用生成的图标与测试文字验证：

- 原生窗口正常显示，Wails Go 方法与事件桥接工作。
- 新建多行中文便签、保存、回到挂绳。
- 通过系统文件选择器导入 PNG，显示缩略图。
- 点击图片显示原图预览及正确的 1024 × 1024 尺寸，关闭后恢复挂绳。
- 收起为 80 × 80 窗口中的小篮子，显示数量；点击可展开。
- 移除卡片与单步撤销，恢复原始时间顺序。
- 自动收集开关可持久化；剪贴板图片变化后新增卡片，来源显示为「剪贴板」。
- 退出并重新启动，卡片与开关设置恢复。
- 设置页与应用内退出入口可用。

实机发现「仅文字剪贴板被当作图片读取失败」的问题，已修复为忽略 `clipboard.ErrNoData` 并继续读取文字；四项回归测试通过。最终修复没有依赖向外部服务发送任何剪贴板内容。

## 内存测量

一次少量卡片的空闲测试，macOS `footprint --noCategories`：

| 进程 | physical footprint |
|---|---:|
| Go / 原生主进程 | 26 MB |
| WebKit WebContent | 17 MB |
| WebKit Networking | 约 6 MB |
| 三个进程合计 | 约 49 MB |

这是单台机器、少量卡片、空闲阶段的一次测量，不是长期常驻或大批图片的保证。对应 RSS 合计约 158 MB；RSS 包含共享页面，不能与 physical footprint 混为一谈。较早一次图片操作后的 RSS 合计约 202 MB，亦不是同一指标的前后对照。

代码设置 Go 64 MiB 软内存预算，内容变化后空闲 5 秒归还可回收 Go 堆页面；这不是整个应用的硬上限。大图解码、原图预览、WebView 与原生框架仍会增加峰值，尚未做长期/压力测试。

## 明确尚未验证或实现

- macOS 交互区域截图的系统授权流程与实际截取结果尚未实机验证。按钮调用系统 `screencapture -i`，拒绝权限时给出提示。
- Windows 的 WebView2 安装状态、系统托盘、区域截图、剪贴板格式兼容性尚需真机验证。
- Intel Mac 已构建，未在 Intel 机器运行。
- 微信 / 钉钉本体未做逐个应用测试；已验证通用系统剪贴板图片收集链路。仅保存文件而未复制的截图需手动导入。
- Windows 系统截图取消后可能等待最多 90 秒；可用系统托盘手动展开。当前没有即时取消回调。
- 多屏选屏、独立全屏应用上方展示、开机自启、OCR、标注、云同步没有实现。全局快捷键已在 0.4.0 加入（见文末）。
- 移除卡片是索引层面移除，图片原文件暂保留在本机，不自动清理。

## Jev 实际使用范围

仅一次批量调用，输入为不含凭证、源码、日志、截图或便签的产品概要。两个 Choice 结果：`local_shelf`（本地收纳首版）、`top_strip`（横向浅挂绳），置信度均为 1.0。Jev 只提供分类建议；设计、复杂架构、代码与验证由 Codex 完成。应用内没有 Jev 或其他在线 AI 请求。

## 0.1.1 图标改版验证

- 单独更新应用 / 常驻入口 / 系统托盘图标；原版 0.1.0 压缩包保留。
- 检查 16 / 32 / 64 / 256 像素渲染及 PNG / ICO / ICNS 导出尺寸。
- 本地浏览器实测收起后显示新图标和数量角标，点击能展开挂绳，效果见 `resident-icon-preview.png`。
- macOS 通用包重新构建并校验本地签名；Windows x64 重新构建并检查 PE 中的图标资源 ID 3。Windows 图标实际显示仍待真机验证。
- Jev 对本次修改范围和视觉风格做了两项 Choice 建议，未发送截图、源码或用户内容。

## 0.3.0 主题、历史、滚动截图与日程

- Go race 测试、`go vet ./...`、前端语法与日程日期边界测试通过。
- 主题 / 字号旧数据兼容、持久化、历史归档后重启恢复、重复恢复去重、旧记录合并、保存失败回滚、损坏主文件备份恢复通过测试。
- 已将早期隔离目录的 4 条记录合并到本机正式目录。原目录未改动；随后正常重开应用，原有记录及用户新加记录保留。正式实例当前版本为 0.3.0。
- 本机 macOS 原生窗口已打开日程入口并显示当天 / 本周 / 本月、新建日程控件。日程保存、编辑、完成状态、跨天校验与重启恢复通过后端测试；跨月周范围、闰年、月末和本地日期边界通过 JavaScript 测试。
- 主题切换、最大字号、历史中文搜索、从历史重新挂上的浏览器交互已检查。
- 系统 NSWorkspace 读取的应用图标已确认是青蓝色新版图标，见 `system-app-icon.png`。运行时也明确设置 NSApplication 图标。未清理系统 Dock 缓存或重启 Dock。
- macOS 滚动截图的主屏幕读取与全屏框选覆盖层已在隔离实例运行。针对“点击后不能滚动”的反馈，改为框选松开鼠标即自动进入小控制台；框选提示明确下一步回到目标应用滚动。
- 拼接模块通过像素级自动重叠拼接、手动重叠、重复画面、无可靠匹配、区域边界、段数上限测试。尚未完成不同第三方应用、多 DPI 和复杂固定导航页面的全面长图实机兼容测试；Windows 尚待真机测试。
- macOS 通用包 / Windows x64 重新构建。验证记录不把编译通过当作 Windows 实机通过。
- 正式用户的数据和截图不放入交付源码包；当前正式实例保持运行，隔离测试实例已退出。
- Jev 仅用于优先级分类与主题范围筛选，没有接收用户便签、日程、截图或凭证。

## 0.4.0 截图权限、动画与体验优化

- `go test -race ./...`、`go vet`（macOS 与 Windows 交叉）通过。新增测试：滚动截图的自动采集（小幅移动暂存、超过 40% 立即拼接、用暂存帧衔接、连续对不上时提示、完成时补上暂存帧）；图片按原始字节和存储字节快速去重；归档图片字节再次出现时恢复记录；永久删除只作用于历史且删除文件；篮子位置与历史数量在重启后保留。
- 隔离实例实测：激活访达后让挂绳空闲自动收起，前台应用仍是访达；收起 / 展开的 Genie 动画正常；篮子位置写入 `shelf.json`。
- ⌃⌥A / ⌃⌥S 在本机 Carbon 注册成功、可重复注册与注销。按键触发、自动长截图的实际滚动拼接、`screencapture -R` 区域采集需要屏幕录制权限，尚未在本机实测。
- 浏览器演示中确认：新增卡片只有该卡片播放动画、旧卡片节点被复用；历史永久删除需两次点击；设置页显示全局快捷键开关；滚动截图控制台在 410 × 250 尺寸下排版正常。
- Windows 全局快捷键与区域采集仅交叉编译，未在 Windows 真机运行。

- 小篮子整体可拖动、挂绳边缘可调整大小：浏览器演示中确认移动超过 4px 的按压不会展开篮子、普通单击仍展开；`shelf.json` 的挂绳尺寸与位置保存与清除有单元测试。macOS 正式实例中，手动拖边调整后的挂绳尺寸在收起时写入 `shelf.json`。篮子在 macOS 原生窗口里的拖动、Windows 上的边缘调整尚未由测试脚本实测。
- Windows 拖出图片修复：系统拖放改为跟随鼠标键的真实状态，并共享窗口线程的输入，不再等一个收不到的松开而卡住。拖动卡住期间挂绳停止刷新，所以移除的卡片看起来还在；现在拖动结束、出错或下一次按下鼠标时都会补画。浏览器演示中确认：模拟卡住时移除卡片不刷新，按下鼠标后立即消失。拖到微信 / QQ / 钉钉聊天框尚需 Windows 真机验证。


## 0.4.1 窗口位置、托盘常驻与退出（2026-10-28）

- `go test -race ./...` 和 macOS `go vet ./...` 通过。新增回归覆盖：保存并重开后的篮子坐标、截图/面板/挂绳不覆盖篮子位置、普通屏/150% DPI/负坐标副屏/顶端与侧边任务栏/显示器断开后的边界回退、系统退出不被拦截且不等待截图锁、后台上下文取消。
- 前端 `tests/quit.test.cjs` 验证设置保存成功、失败、永久挂起三种情况下均会调用退出，等待正常设置保存但最多 250ms；退出调用本身的错误仍报告。`tests/idle.test.cjs`、`tests/schedule_dates.test.cjs` 及 JS 语法检查通过。
- macOS 使用独立 bundle ID 与独立数据目录的测试版，关闭自动收集和快捷键；实际点击设置中的「退出拾光绳」和按 Cmd+Q 后，进程均结束。测试未修改正式用户记录。
- macOS arm64/amd64 通用应用与 Windows amd64 可执行文件重新构建。Windows 未执行、没有真机显示验证；原生位置与 DPI、通知区图标和任务栏行为仍需 Windows 10/11 实机验收，不能把编译和坐标逻辑测试当成实机通过。
- Windows 完整 `go vet` 仍报告原有 `drag_windows.go` COM 指针桥接的 `unsafe.Pointer` 告警；未改写此第三方更新的拖放实现。关闭该单项分析后的 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet -unsafeptr=false ./...` 通过。本轮新增 Windows 窗口代码没有该告警。
- Jev 仅对脱敏问题描述做两项分类：位置问题路由到布局/持久化，任务栏问题路由到 Windows 原生窗口集成；未提交源码、用户记录、截图或凭证。代码与验证由 Codex 完成。

## Public documentation — 2026-10-30

- Added Simplified Chinese, Traditional Chinese, English and Japanese project introductions and user guides. These are documentation translations, not application UI localization.
- Captured six frontend browser-demo screenshots using synthetic cards and newly created example schedules. Public media provenance is recorded in `docs/media/README.md`; existing private native screenshots are excluded from publication.
- Added MIT license with user approval, sharing copy, issue/PR templates and build instructions. No new runtime network calls or analytics were added.
- Jev classified the documentation audience and safe asset source from a minimal product description only; no source code, personal data or credentials were sent.
