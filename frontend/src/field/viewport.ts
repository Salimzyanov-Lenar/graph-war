export interface Point {
  x: number;
  y: number;
}

export interface ViewportOptions {
  worldWidth: number;
  worldHeight: number;
  screenWidth: number;
  screenHeight: number;
  padding?: number;
}

export interface Viewport {
  scale: number;
  offsetX: number;
  offsetY: number;
  worldToScreen(point: Point): Point;
  screenToWorld(point: Point): Point;
}

/** Мир: x вправо, y вверх, начало в левом нижнем углу. Экран: y вниз. Единицы экрана — CSS-пиксели. */
export function createViewport({
  worldWidth,
  worldHeight,
  screenWidth,
  screenHeight,
  padding = 0,
}: ViewportOptions): Viewport {
  const availableWidth = Math.max(screenWidth - 2 * padding, 0);
  const availableHeight = Math.max(screenHeight - 2 * padding, 0);
  const scale = Math.min(availableWidth / worldWidth, availableHeight / worldHeight);
  const offsetX = (screenWidth - worldWidth * scale) / 2;
  const offsetY = (screenHeight - worldHeight * scale) / 2;

  return {
    scale,
    offsetX,
    offsetY,
    worldToScreen: ({ x, y }) => ({
      x: offsetX + x * scale,
      y: offsetY + (worldHeight - y) * scale,
    }),
    screenToWorld: ({ x, y }) => ({
      x: (x - offsetX) / scale,
      y: worldHeight - (y - offsetY) / scale,
    }),
  };
}
