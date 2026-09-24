# EPIC-045 — Global Player

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P0

## Цель

Плеер: UI → Player Store → Controller → Audio Engine → HTMLAudioElement. Аудио идёт из CDN по signed URL.

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-045.1 Zustand Player Store: queue, queueIndex, currentTrack, isPlaying, volume, muted, position, duration, shuffle, repeatMode, buffering, error.

- [x] TASK-045.2 AudioEngine: `HtmlAudioEngine` (signed URL) и `SimulatedAudioEngine` (mock-режим без media origin).

- [x] TASK-045.3 Controller: загрузка трека, stale-grant защита, play/pause/seek/volume, события engine → store.

- [x] TASK-045.4 PlayerBar (CPlayerBar): idle/playing/paused/buffering, shuffle/repeat, seek, volume; mobile mini-player.

- [x] TASK-045.5 Play для карточек/строк: `usePlayCollection`, `usePlayTrackList`, playing-состояния карточек и строк.

- [ ] TASK-045.6 Stream Authorization API (EPIC-017) вместо mock `/playback/tracks/{id}/stream`.

- [ ] TASK-045.7 Queue drawer, like, device picker, expanded player (нужны макеты).

- [ ] TASK-045.8 Media Session API, горячие клавиши, сохранение громкости.

## Definition of Done

Плеер переживает навигацию; логика очереди покрыта unit-тестами.

---
