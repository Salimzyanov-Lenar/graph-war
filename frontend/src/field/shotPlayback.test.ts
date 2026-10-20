import { describe, expect, it } from 'vitest';
import { pathLength, shotDurationMs, shotProgress, visiblePath, type PathPoint } from './shotPlayback';

const straight: PathPoint[] = [[0, 0], [30, 0], [60, 0]];
const bent: PathPoint[] = [[0, 0], [10, 0], [10, 10]];

describe('visiblePath', () => {
  it('на нуле показывает только начало, на единице — всю ломаную ровно до последней серверной точки', () => {
    expect(visiblePath(straight, 0)).toEqual([{ x: 0, y: 0 }]);
    expect(visiblePath(straight, 1)).toEqual([{ x: 0, y: 0 }, { x: 30, y: 0 }, { x: 60, y: 0 }]);
    expect(visiblePath(straight, 5)).toEqual(visiblePath(straight, 1));
  });

  it('в середине обрезает сегмент по длине и не дорисовывает дальше', () => {
    expect(visiblePath(straight, 0.25)).toEqual([{ x: 0, y: 0 }, { x: 15, y: 0 }]);
    expect(visiblePath(bent, 0.75)).toEqual([{ x: 0, y: 0 }, { x: 10, y: 0 }, { x: 10, y: 5 }]);
  });

  it('одна точка и вырожденные сегменты дают конечные координаты без NaN', () => {
    expect(visiblePath([[3, 4]], 0.5)).toEqual([{ x: 3, y: 4 }]);
    const flat = visiblePath([[1, 1], [1, 1], [1, 1]], 0.5);
    expect(flat.every((p) => Number.isFinite(p.x) && Number.isFinite(p.y))).toBe(true);
    expect(visiblePath([], 0.5)).toEqual([]);
  });

  it('конечная точка одинакова при любой частоте кадров: зависит только от времени', () => {
    const duration = shotDurationMs(straight);
    const end = (frames: number) => {
      let last = visiblePath(straight, 0);
      for (let i = 1; i <= frames; i += 1) last = visiblePath(straight, shotProgress(0, (duration * i) / frames, duration));
      return last;
    };
    expect(end(5)).toEqual(end(240));
  });
});

describe('время и длительность', () => {
  it('прогресс ограничен [0, 1], в том числе после долгой фоновой паузы', () => {
    expect(shotProgress(1000, 900, 500)).toBe(0);
    expect(shotProgress(1000, 1250, 500)).toBe(0.5);
    expect(shotProgress(1000, 600_000, 500)).toBe(1);
  });

  it('длительность укладывается в окно resolving и растёт с длиной пути', () => {
    expect(shotDurationMs([[0, 0], [1, 0]])).toBe(350);
    expect(shotDurationMs([[0, 0], [1000, 0]])).toBe(1000);
    expect(shotDurationMs(straight)).toBeGreaterThan(shotDurationMs([[0, 0], [30, 0]]) - 1);
    expect(pathLength(bent)).toBe(20);
  });
});
