# EPIC-041 — Frontend Foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Поднять `apps/web` в Bun workspace на стеке specs: React + TypeScript + Vite.

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-041.1 Root `package.json` с Bun workspaces (`apps/*`), `bun.lock`; npm/pnpm/yarn lockfiles запрещены (.gitignore).

- [x] TASK-041.2 `apps/web`: Vite, React, TypeScript (strict), алиас `@/`.

- [x] TASK-041.3 React Router, TanStack Query, Zustand, Zod.

- [x] TASK-041.4 FSD-структура: `app/ pages/ widgets/ features/ entities/ shared/`.

- [x] TASK-041.5 API-граница: единый клиент `/api/v1` (Vite proxy → gateway), error model, zod-валидация ответов.

- [x] TASK-041.6 Mock API отдельно от UI (`shared/api/mock`), включается `VITE_API_MOCKS`; в prod-бандл не попадает (lazy chunk).

- [x] TASK-041.7 ESLint (typescript-eslint, react-hooks, запрет raw HEX в TS), Vitest + RTL.

- [ ] TASK-041.8 React Hook Form — подключить вместе с первой формой (Auth UI / Create playlist).

## Definition of Done

`bun install`, `bun run build`, `bun run test`, `bun run lint` проходят.

---
