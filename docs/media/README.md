# Public media / 公开图片

Captured or created on 2026-10-30.

- `cover.svg`, `cover.png`, `social.png`: original vector marketing artwork,
  rasterized from the editable SVG. These are illustrations, not screenshots.
- `overview.jpg`, `settings.jpg`, `dark.jpg`, `history.jpg`, `schedule.jpg`,
  `basket.jpg`: screenshots of the actual frontend in `frontend/preview.html`,
  captured through browser UI with fresh sample data. The footer explicitly
  labels it an interactive demo with sample content.
- Screenshot examples: generated landscape/chart/palette cards, generic notes,
  and two newly entered reading/creative-review schedules. No production history,
  personal screenshots, or user schedules were used.
- Browser demo does not implement native tray, system capture, or outbound native
  file drag. These images must not be presented as proof of Windows integration.
- The demo shelf is resized for readable examples. Product default height remains
  approximately 20% of the available display height and is user-adjustable.

To reproduce: run the local server in `docs/BUILD.md`, open the demo, reset its
sample state, and use the normal UI to open settings/history/schedules or collapse.
Use a new browser origin/session and never import a real `shelf.json`.
