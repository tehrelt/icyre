# design/

`icyre-tokens.json` — **копия** `project/tokens.json` из ICYRE Design System
(https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT). Это единственный источник значений токенов во frontend.

```bash
bun run tokens   # icyre-tokens.json → src/app/styles/tokens.css
```

Не редактируйте `tokens.css` руками: обновите токены в Design System, скопируйте свежий
`tokens.json` сюда и перегенерируйте. Стили компонентов (`src/shared/ui/styles/components.css`)
перенесены из `project/components/bundle.css` той же Design System.
