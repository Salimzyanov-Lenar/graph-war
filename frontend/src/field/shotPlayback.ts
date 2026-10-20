import type { Point } from './viewport';

/** Скорость «снаряда» только для показа; на исход выстрела она не влияет. */
const SPEED = 70;
const MIN_DURATION_MS = 350;
/** Меньше серверной фазы resolving (1200 мс), чтобы анимация успевала закончиться до следующего хода. */
const MAX_DURATION_MS = 1000;

export type PathPoint = readonly [number, number];

export function pathLength(points: readonly PathPoint[]): number {
  let total = 0;
  for (let i = 1; i < points.length; i += 1) {
    const a = points[i - 1]!;
    const b = points[i]!;
    total += Math.hypot(b[0] - a[0], b[1] - a[1]);
  }
  return total;
}

export function shotDurationMs(points: readonly PathPoint[]): number {
  const ms = (pathLength(points) / SPEED) * 1000;
  return Math.min(MAX_DURATION_MS, Math.max(MIN_DURATION_MS, ms));
}

/** Прогресс определяется по часам, а не по числу кадров: на любой частоте и после фоновой вкладки итог один. */
export function shotProgress(startedAt: number, now: number, durationMs: number): number {
  if (durationMs <= 0) return 1;
  return Math.min(1, Math.max(0, (now - startedAt) / durationMs));
}

/** Часть серверной ломаной до доли `progress` её длины. За последнюю серверную точку не выходит. */
export function visiblePath(points: readonly PathPoint[], progress: number): Point[] {
  const first = points[0];
  if (!first) return [];
  const result: Point[] = [{ x: first[0], y: first[1] }];
  const target = pathLength(points) * Math.min(1, Math.max(0, progress));
  let walked = 0;
  for (let i = 1; i < points.length; i += 1) {
    const a = points[i - 1]!;
    const b = points[i]!;
    const segment = Math.hypot(b[0] - a[0], b[1] - a[1]);
    if (walked + segment <= target) {
      result.push({ x: b[0], y: b[1] });
      walked += segment;
      continue;
    }
    const t = segment === 0 ? 0 : (target - walked) / segment;
    if (t > 0) result.push({ x: a[0] + (b[0] - a[0]) * t, y: a[1] + (b[1] - a[1]) * t });
    break;
  }
  return result;
}
