# Graphwar — frontend

TypeScript + Vite + React + PixiJS. React владеет DOM-интерфейсом, PixiJS — одной игровой сценой. Правила матча, траектории и победа приходят с сервера; клиент их не вычисляет. Задачи — [tasks.md](tasks.md).

## Требования

- Node.js `>=22.12.0` (проверено на 24.13.1)
- npm (версии зависимостей зафиксированы точно, lockfile — `package-lock.json`)

## Команды

```bash
npm install        # установка зависимостей по lockfile
npm run dev        # dev-сервер Vite
npm run typecheck  # tsc --noEmit (Vite типы не проверяет)
npm test           # Vitest, окружение jsdom
npm run build      # production-сборка в dist/
npm run preview    # просмотр production-сборки
```

## Устройство сцены

- `src/scene/mountScene.ts` — адаптер жизненного цикла `Application` из PixiJS: асинхронный `init`, unmount до его завершения, повторный mount, уничтожение приложения и удаление canvas.
- `src/scene/GameCanvas.tsx` — React-компонент: создаёт сцену в `useEffect` и освобождает её в cleanup. Покадровая анимация идёт через ticker Pixi, не через React state.
- `src/scene/demoScene.ts` — временная локальная демонстрационная сцена (вращающийся квадрат), не похожая на готовый матч.

React Strict Mode включён в `src/main.tsx` и не отключается: тесты проверяют, что после двойного эффекта остаётся один canvas.

## Не сделано в F01

Playwright smoke-тест пока не добавлен: запуск Playwright по договорённости только по явной команде владельца.
