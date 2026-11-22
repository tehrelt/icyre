# EPIC-051 — Auth UI

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Цель

Вход/регистрация.

Design System: `design-system/` · Макет: `design-system/auth-preview.html`

## Задачи

- [x] TASK-051.1 Подключить `react-hook-form` + `@hookform/resolvers` + Zod-схемы (`loginSchema`, `registerSchema`) по контракту Auth Service (`services/auth`).

- [x] TASK-051.2 Реализовать `features/auth` (`AuthForm`, `useLoginMutation`, `useRegisterMutation`, `useLogoutMutation`, `mapAuthError`, индикатор надёжности пароля, toggle видимости пароля) и mock-маршруты `POST /auth/login`, `POST /auth/register`.

- [x] TASK-051.3 Реализовать экран `AuthPage` (`/login`, `/register`) по макету `design-system/auth-preview.html` на компонентах DS (`Logo`, `SegmentedControl`, `Input`, `Checkbox`, `Button`, `FeaturedCard`, `PlaybackState`), меню аккаунта / кнопку `Sign in` в `PageToolbar`.

- [x] TASK-051.4 Покрыть тестами валидацию, маппинг ошибок, вход, регистрацию и выход (`AuthPage.test.tsx`, `authApi.test.ts`).

## Definition of Done

Экраны `/login` и `/register` работают с Auth API и mock-сервером, `bun run typecheck`, `bun run lint`, `bun run test`, `bun run build` проходят.

---
