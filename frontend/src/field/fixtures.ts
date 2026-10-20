import type { FieldState } from './types';

/** Ручная фикстура формата контракта v0.1: поле 100×60, три бойца на сторону. */
export const sampleField: FieldState = {
  width: 100,
  height: 60,
  obstacles: [
    { x: 50, y: 30, r: 8 },
    { x: 30, y: 12, r: 4 },
    { x: 72, y: 48, r: 5 },
    { x: 62, y: 10, r: 3 },
  ],
  fighters: [
    { id: 'a1', side: 'A', x: 8, y: 14, alive: true },
    { id: 'a2', side: 'A', x: 12, y: 30, alive: true },
    { id: 'a3', side: 'A', x: 8, y: 48, alive: false },
    { id: 'b1', side: 'B', x: 92, y: 12, alive: true },
    { id: 'b2', side: 'B', x: 88, y: 30, alive: true },
    { id: 'b3', side: 'B', x: 92, y: 46, alive: true },
  ],
  selectedFighterId: 'a2',
};
