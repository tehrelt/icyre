# @icyre/web

React + TypeScript + Vite на Bun. Визуальный язык — **ICYRE Design System**, экраны — продуктовый canvas.

```bash
bun install          # из корня monorepo
bun run dev          # http://localhost:5173
bun run build        # tsc -b && vite build
bun run test         # Vitest + React Testing Library
bun run lint
bun run tokens       # design/icyre-tokens.json → src/app/styles/tokens.css
```

## Архитектура

```text
src/app/        App, providers (Query, Player), router, AppShell layout, global styles (tokens.css, canvas-tokens.css, base.css)
src/pages/      home (canvas Main), not-built (заглушка для экранов следующих эпиков)
src/widgets/    sidebar, page-toolbar, player-bar, section-header, quick-tile, mobile-nav (shell-компоненты canvas)
src/features/   player: store (Zustand) → controller → AudioEngine → HTMLAudioElement
src/entities/   track, album, artist, playlist, user, library — zod-схемы и queries
src/shared/     ui (компоненты DS), api (клиент /api/v1 + mock), config, lib, hooks
```

- **Server state** — TanStack Query; **client state** — Zustand (плеер). Server state в Zustand не хранится.
- **Tokens**: только `var(--token)`. Семантические токены DS; значения из макетов, которых нет в DS, — в `canvas-tokens.css`.
- **API**: браузер ходит только в `/api/v1` (Vite proxy → gateway). `VITE_API_MOCKS=true` (по умолчанию в dev)
  обслуживает запросы из `shared/api/mock` — контракты те же, что у будущего BFF.
- **Плеер** живёт выше `<Outlet />` и не размонтируется при навигации. Аудио берётся по signed URL из CDN;
  в mock-режиме — `SimulatedAudioEngine` (тихие часы), чтобы UX плеера работал без media origin.

## Переменные окружения

См. `.env.example`.
