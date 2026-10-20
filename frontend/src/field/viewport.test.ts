import { describe, expect, it } from 'vitest';
import { pickFighter } from './hitTest';
import { sampleField } from './fixtures';
import { createViewport } from './viewport';

const world = { worldWidth: 100, worldHeight: 60 };

describe('createViewport', () => {
  it('переводит углы поля и центр в экранные координаты с y вниз', () => {
    const view = createViewport({ ...world, screenWidth: 1000, screenHeight: 600 });

    expect(view.scale).toBe(10);
    expect(view.worldToScreen({ x: 0, y: 0 })).toEqual({ x: 0, y: 600 });
    expect(view.worldToScreen({ x: 100, y: 60 })).toEqual({ x: 1000, y: 0 });
    expect(view.worldToScreen({ x: 50, y: 30 })).toEqual({ x: 500, y: 300 });
  });

  it('сохраняет пропорции и центрирует поле (letterbox)', () => {
    const wide = createViewport({ ...world, screenWidth: 1400, screenHeight: 600 });
    expect(wide.scale).toBe(10);
    expect(wide.offsetX).toBe(200);
    expect(wide.offsetY).toBe(0);

    const tall = createViewport({ ...world, screenWidth: 500, screenHeight: 600 });
    expect(tall.scale).toBe(5);
    expect(tall.offsetX).toBe(0);
    expect(tall.offsetY).toBe(150);
  });

  it('учитывает отступы', () => {
    const view = createViewport({ ...world, screenWidth: 1100, screenHeight: 700, padding: 50 });
    expect(view.scale).toBe(10);
    expect(view.worldToScreen({ x: 0, y: 60 })).toEqual({ x: 50, y: 50 });
  });

  it('обратимо: screenToWorld(worldToScreen(p)) возвращает исходную точку', () => {
    for (const [w, h] of [[1000, 600], [377, 913], [1920, 1080]] as const) {
      const view = createViewport({ ...world, screenWidth: w, screenHeight: h, padding: 12 });
      for (const point of [{ x: 0, y: 0 }, { x: 100, y: 60 }, { x: 50, y: 30 }, { x: 13.37, y: 42.1 }]) {
        const back = view.screenToWorld(view.worldToScreen(point));
        expect(back.x).toBeCloseTo(point.x, 9);
        expect(back.y).toBeCloseTo(point.y, 9);
      }
    }
  });

  it('не выдаёт отрицательный масштаб на слишком маленьком экране', () => {
    const view = createViewport({ ...world, screenWidth: 20, screenHeight: 20, padding: 50 });
    expect(view.scale).toBe(0);
  });
});

describe('pickFighter', () => {
  it('не зависит от размера окна: один экранный клик по бойцу на разных размерах даёт того же бойца', () => {
    const target = sampleField.fighters.find((f) => f.id === 'b2')!;
    for (const [w, h] of [[1000, 600], [640, 480], [1920, 1080]] as const) {
      const view = createViewport({ ...world, screenWidth: w, screenHeight: h, padding: 16 });
      const screen = view.worldToScreen(target);
      const picked = pickFighter(sampleField.fighters, view.screenToWorld(screen));
      expect(picked?.id).toBe('b2');
    }
  });

  it('возвращает undefined вдали от бойцов и ближайшего при перекрытии', () => {
    expect(pickFighter(sampleField.fighters, { x: 50, y: 5 })).toBeUndefined();
    const fighters = [
      { id: 'p', side: 'A', x: 10, y: 10, alive: true },
      { id: 'q', side: 'B', x: 10.8, y: 10, alive: true },
    ] as const;
    expect(pickFighter(fighters, { x: 10.7, y: 10 })?.id).toBe('q');
  });
});
