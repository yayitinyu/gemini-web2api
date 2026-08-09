# Admin design specification

The production UI follows three generated references:

- `admin-overview-concept.png` — desktop overview, native 1536×1024
- `admin-login-concept.png` — desktop password login, native 1536×1024
- `admin-mobile-concept.png` — mobile overview, native 390×844

## Direction

The visual idea is a midnight observatory for infrastructure: an ink-indigo
canvas, cold-white information, and one violet-to-cyan signal trace. Layout is
open and asymmetrical. The desktop navigation is a detached left rail; mobile
uses a safe-area-aware bottom island. The UI avoids a generic card grid.

## Tokens

- Canvas: `#050713`; elevated canvas: `#090d1c`; panel core: `#0c1122`
- Text: `#f7f8ff`; secondary: `#a8aec5`; quiet: `#717991`
- Accent start: `#8b6cff`; accent end: `#47d8ff`
- Success: `#55e68a`; danger: `#ff626f`; warning: `#ffc565`
- Hairline: translucent blue-white, never an opaque gray border
- Display/UI family: Plus Jakarta Sans Variable
- Editorial accents: Newsreader Variable
- Radius scale: 12, 18, 26, 34px; major panels use a concentric double bezel
- Motion: 180–760ms, custom `cubic-bezier(0.32, 0.72, 0, 1)`; transform and
  opacity only; all nonessential motion disabled under `prefers-reduced-motion`

## Locked desktop overview copy

Brand `Gemini Web2API`; navigation `概览`, `账号`, `网络`, `请求`, `设置`;
title `运行概览`; status `网关在线`; action `连通性检测`; KPIs `24 小时请求`,
`成功率`, `P50 延迟`, `输出 Tokens`; sections `请求脉冲`, `接入信息`,
`账号健康`, `最近请求`.

Fresh installations show `0` or `—`; populated values always come from the
admin API. Prompt and response bodies never appear in the dashboard.

## Component inventory

- App shell, detached navigation rail, mobile bottom navigation
- Brand mark, status signal, precise line-icon buttons, toast and dialog
- Typographic KPI band, request pulse chart, recent-request table
- Connection panel with masked key, copy and rotation flows
- Account and proxy lists with health states and edit dialogs
- Request filters and pagination
- Runtime settings form
- Password login with reveal, error, loading, and keyboard-focus states

Icons use the Phosphor light visual language at a consistent optical size.
Arrows and the brand mark are project-local SVG components.

## Responsive rules

Below 768px the desktop rail disappears, utility panels become open action
rows, the KPI band becomes a two-column layout, rotations/overlaps are removed,
and the bottom island observes `env(safe-area-inset-bottom)`. All touch targets
are at least 44px. Long endpoints truncate without causing horizontal overflow.
