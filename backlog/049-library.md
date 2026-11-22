# EPIC-049 — Library

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Цель

Экран библиотеки.

Design System: `design-system/` · Макет: `design-system/library-preview.html`

## Задачи

- [x] TASK-049.1 Спроектировать макет библиотеки и Liked tracks в `design-system/library-preview.html` на компонентах DS.

- [x] TASK-049.2 Реализовать API-слой `entities/library` (`fetchSavedAlbumsPage`, `fetchLikedTracksPage`, `fetchFollowedArtistsPage`, `setAlbumSaved`, `setArtistFollowed`, `useSavedAlbumsContains`) и фичи `features/save-album`, `features/follow-artist` с оптимистичным состоянием и откатом.

- [x] TASK-049.3 Реализовать экраны `LibraryPage` (`/library`, `/liked`) с вкладками (`All`, `Playlists`, `Albums`, `Artists`, `Liked tracks`), фильтрацией, сортировкой, пагинацией, spotlight-карточкой Liked tracks, контекстными меню и состояниями loading / signed-out / empty / error; подключить сохранение альбома на `AlbumPage` и переход `Manage` на `HomePage`.

- [x] TASK-049.4 Добавить mock-маршруты Library/Social/Catalog в `shared/api/mock/server.ts` и покрыть тестами (`LibraryPage.test.tsx`, `AlbumPage.test.tsx`, `HomePage.test.tsx`).

## Definition of Done

Экраны `/library` и `/liked` работают с Library/Playlist/Social/Catalog API и mock-сервером, `bun run typecheck`, `bun run lint`, `bun run test`, `bun run build` проходят.

---
