# Visual fidelity and browser QA ledger

Reviewed on 2026-08-09 against the three generated design concepts in this directory.

## Comparison ledger

| Area | Concept intent | Production implementation | Result |
| --- | --- | --- | --- |
| Composition | Detached desktop rail and asymmetrical content field | Fixed double-bezel rail; open KPI band; chart and access panel use unequal columns | Match |
| Color system | Ink-indigo canvas, cold white type, violet-to-cyan signal | Production CSS uses the locked tokens from `spec.md`; green is reserved for health | Match |
| Typography | High-impact sans display with editorial contrast | Plus Jakarta Sans drives UI/display; Newsreader is limited to metric/editorial accents | Match |
| Data honesty | Fresh installs show `0` or `—`, never invented demo values | Every metric comes from `/api/admin/overview`; empty states remain explicit | Match |
| Signal visualization | A luminous request trace inside a double bezel | SVG trace uses live hourly buckets; a one-bucket edge case now renders a real single pulse | Match |
| Access surface | Endpoint, masked key, and key management near the chart | Endpoint and key hint are exposed without returning secret plaintext | Match |
| Responsive behavior | Safe-area mobile bottom island and no desktop rail | 390×844 check: five 44px navigation targets, rail hidden, no horizontal overflow | Match with one intentional addition |
| Motion | Fluid but restrained, reduced-motion aware | Page/modal/toast transitions use the shared easing; reduced motion disables them | Match |
| Iconography | One precise line-icon family, no emoji | Direct Phosphor light icon imports plus one local brand SVG | Match |

The mobile concept showed four destinations. Production includes a fifth, **网络**, because proxy egress is a first-class operational surface and hiding it behind settings would weaken information architecture.

## Above-the-fold copy diff

The overview's locked copy is preserved: `运行概览`, `网关在线`, `连通性检测`, the four KPI names, `请求脉冲`, `接入信息`, `账号健康`, and `最近请求`.

The login concept's generic `欢迎回来` was intentionally replaced with the more product-specific editorial line `把网页能力，折叠成接口。`; `进入控制台` became `验证并进入` to make the authentication action explicit. The implementation also states that credentials stay on the server and that the session cookie is not exposed to page scripts.

## Browser evidence

- Desktop default viewport: 1280×720; complete overview captured at 1265×1172.
- Mobile override: 390×844; `scrollWidth <= innerWidth`, desktop rail hidden, five bottom actions visible.
- Password login reached `/admin/overview` with a real HttpOnly session.
- Accounts, network, requests, and settings navigation all resolved to their expected headings.
- The account editor required a Cookie for new records and closed normally.
- The request table showed both live integration probes.
- Environment-managed API key correctly disabled panel rotation.
- Browser console after the full flow: zero warnings and zero errors.

Production captures:

- `../screenshots/admin-login.png`
- `../screenshots/admin-overview.png`
- `../screenshots/admin-mobile.png`
