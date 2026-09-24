# Audio Analysis Worker

## Назначение
Извлечение признаков для рекомендаций и аналитики.

## Возможные признаки
- BPM;
- loudness;
- duration;
- spectral features;
- silence ratio.

## Ограничение
Для дипломной реализации достаточно BPM/loudness и нескольких простых характеристик.

## Event
```text
audio.features_extracted
```
