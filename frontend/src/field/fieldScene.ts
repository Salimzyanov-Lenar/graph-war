import { Application, Container, Graphics, Text, type FederatedPointerEvent } from 'pixi.js';
import { mountScene, type SceneHandle, type Teardown } from '../scene/mountScene';
import { sampleField } from './fixtures';
import { FIGHTER_RADIUS, pickFighter } from './hitTest';
import type { FieldState, Fighter, Side } from './types';
import { createViewport, type Viewport } from './viewport';

const PADDING = 24;
const GRID_STEP = 10;
const COLORS = {
  field: 0x172033,
  grid: 0x2a3550,
  axis: 0x6f7fa8,
  border: 0xaab6d6,
  obstacle: 0x56627f,
  obstacleEdge: 0x8793b3,
  dead: 0x6b7280,
  selection: 0xffd24a,
  hover: 0xffffff,
  side: { A: 0x4f8cff, B: 0xff7a59 } satisfies Record<Side, number>,
};

export interface FieldSceneOptions {
  onFighterClick?: (fighter: Fighter) => void;
}

export function setupFieldScene(app: Application, state: FieldState, options: FieldSceneOptions = {}): Teardown {
  const graphics = new Graphics();
  const labels = new Container();
  app.stage.addChild(graphics, labels);
  app.stage.eventMode = 'static';
  app.stage.cursor = 'default';

  let viewport: Viewport = makeViewport();
  let hoveredId: string | undefined;

  function makeViewport(): Viewport {
    return createViewport({
      worldWidth: state.width,
      worldHeight: state.height,
      screenWidth: app.screen.width,
      screenHeight: app.screen.height,
      padding: PADDING,
    });
  }

  function draw() {
    viewport = makeViewport();
    app.stage.hitArea = app.screen;
    graphics.clear();
    for (const child of labels.removeChildren()) child.destroy();

    drawField(graphics, viewport, state);
    for (const obstacle of state.obstacles) {
      const center = viewport.worldToScreen(obstacle);
      graphics
        .circle(center.x, center.y, obstacle.r * viewport.scale)
        .fill(COLORS.obstacle)
        .stroke({ width: 2, color: COLORS.obstacleEdge });
    }
    for (const fighter of state.fighters) {
      drawFighter(graphics, labels, viewport, fighter, {
        selected: fighter.id === state.selectedFighterId,
        hovered: fighter.id === hoveredId,
      });
    }
  }

  function fighterAt(event: FederatedPointerEvent): Fighter | undefined {
    const world = viewport.screenToWorld({ x: event.global.x, y: event.global.y });
    return pickFighter(state.fighters, world);
  }

  const onMove = (event: FederatedPointerEvent) => {
    const id = fighterAt(event)?.id;
    if (id === hoveredId) return;
    hoveredId = id;
    app.stage.cursor = id ? 'pointer' : 'default';
    draw();
  };
  const onLeave = () => {
    if (hoveredId === undefined) return;
    hoveredId = undefined;
    app.stage.cursor = 'default';
    draw();
  };
  const onTap = (event: FederatedPointerEvent) => {
    const fighter = fighterAt(event);
    if (fighter) options.onFighterClick?.(fighter);
  };

  app.stage.on('pointermove', onMove);
  app.stage.on('pointerleave', onLeave);
  app.stage.on('pointertap', onTap);
  app.renderer.on('resize', draw);
  draw();

  return () => {
    app.renderer.off('resize', draw);
    app.stage.off('pointermove', onMove);
    app.stage.off('pointerleave', onLeave);
    app.stage.off('pointertap', onTap);
    graphics.destroy();
    labels.destroy({ children: true });
  };
}

function drawField(g: Graphics, view: Viewport, state: FieldState) {
  const topLeft = view.worldToScreen({ x: 0, y: state.height });
  const size = { w: state.width * view.scale, h: state.height * view.scale };
  g.rect(topLeft.x, topLeft.y, size.w, size.h).fill(COLORS.field);

  for (let x = GRID_STEP; x < state.width; x += GRID_STEP) {
    const a = view.worldToScreen({ x, y: 0 });
    const b = view.worldToScreen({ x, y: state.height });
    g.moveTo(a.x, a.y).lineTo(b.x, b.y).stroke({ width: 1, color: COLORS.grid });
  }
  for (let y = GRID_STEP; y < state.height; y += GRID_STEP) {
    const a = view.worldToScreen({ x: 0, y });
    const b = view.worldToScreen({ x: state.width, y });
    g.moveTo(a.x, a.y).lineTo(b.x, b.y).stroke({ width: 1, color: COLORS.grid });
  }

  const origin = view.worldToScreen({ x: 0, y: 0 });
  const right = view.worldToScreen({ x: state.width, y: 0 });
  const top = view.worldToScreen({ x: 0, y: state.height });
  g.moveTo(origin.x, origin.y).lineTo(right.x, right.y).stroke({ width: 2, color: COLORS.axis });
  g.moveTo(origin.x, origin.y).lineTo(top.x, top.y).stroke({ width: 2, color: COLORS.axis });

  g.rect(topLeft.x, topLeft.y, size.w, size.h).stroke({ width: 2, color: COLORS.border });
}

interface FighterFlags {
  selected: boolean;
  hovered: boolean;
}

/** Сторона различается формой (A — круг, B — квадрат), состояние — заливкой/крестом, выбор — кольцом, а не только цветом. */
function drawFighter(g: Graphics, labels: Container, view: Viewport, fighter: Fighter, flags: FighterFlags) {
  const center = view.worldToScreen(fighter);
  const r = Math.max(FIGHTER_RADIUS * view.scale, 6);
  const color = fighter.alive ? COLORS.side[fighter.side] : COLORS.dead;

  if (flags.selected) {
    g.circle(center.x, center.y, r + 6).stroke({ width: 3, color: COLORS.selection });
  }
  if (flags.hovered) {
    g.circle(center.x, center.y, r + 3).stroke({ width: 2, color: COLORS.hover });
  }

  if (fighter.side === 'A') g.circle(center.x, center.y, r);
  else g.rect(center.x - r, center.y - r, 2 * r, 2 * r);

  if (fighter.alive) {
    g.fill(color).stroke({ width: 2, color: 0xffffff });
  } else {
    g.fill({ color, alpha: 0.25 }).stroke({ width: 2, color });
    g.moveTo(center.x - r, center.y - r).lineTo(center.x + r, center.y + r).stroke({ width: 2, color });
    g.moveTo(center.x - r, center.y + r).lineTo(center.x + r, center.y - r).stroke({ width: 2, color });
  }

  const label = new Text({
    text: fighter.id.toUpperCase(),
    style: { fill: fighter.alive ? 0xffffff : COLORS.dead, fontSize: 12, fontFamily: 'system-ui, sans-serif' },
  });
  label.anchor.set(0.5, 1);
  label.position.set(center.x, center.y - r - 4);
  labels.addChild(label);
}

export function mountFieldScene(host: HTMLElement, onError: (error: unknown) => void): SceneHandle {
  return mountScene(host, {
    createApp: () => new Application(),
    setup: (app) => setupFieldScene(app, sampleField),
    onError,
  });
}
