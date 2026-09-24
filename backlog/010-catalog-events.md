# EPIC-010 — Catalog events

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P1

## Задачи

- [ ] TASK-010.1 Добавить event:
  - `track.created`

- [ ] TASK-010.2 Добавить:
  - `track.updated`
  - `album.created`
  - `artist.created`

- [ ] TASK-010.3 Публиковать событие после успешной операции.

- [ ] TASK-010.4 Добавить contract version.

- [ ] TASK-010.5 Добавить tests event mapping.

## Definition of Done

- После создания Track Kafka получает событие.
- Event payload не зависит от HTTP DTO.
- Event version указан явно.

---
