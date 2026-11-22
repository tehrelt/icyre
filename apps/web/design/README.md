# design/

`icyre-tokens.json` — **копия** `design-system/tokens.json` из локальной ICYRE Design System
(`design-system/`). Это единственный источник значений токенов во frontend.

```bash
bun run tokens   # icyre-tokens.json → src/app/styles/tokens.css
```

Не редактируйте `tokens.css` руками: обновите токены в Design System, скопируйте свежий
`tokens.json` сюда и перегенерируйте. Стили компонентов (`src/shared/ui/styles/components.css`)
перенесены из `project/components/bundle.css` той же Design System.
