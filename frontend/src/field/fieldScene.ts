import { Application, Container, Graphics, Text, type FederatedPointerEvent } from 'pixi.js';
import { mountScene, type SceneHandle, type Teardown } from '../scene/mountScene';
import { sampleField } from './fixtures';
import { FIGHTER_RADIUS, pickFighter } from './hitTest';
import { shotDurationMs, shotProgress, visiblePath } from './shotPlayback';
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
  shot: 0xffe08a,
  impact: 0xff5d5d,
  hover: 0xffffff,
  side: { A: 0x4f8cff, B: 0xff7a59 } satisfies Record<Side, number>,
};

export interface FieldSource {
  getState(): FieldState | undefined;
  subscribe(listener: () => void): () => void;
  onFighterClick?(fighter: Fighter): void;
}

export function staticFieldSource(state: FieldState): FieldSource {
  return { getState: () => state, subscribe: () => () => {} };
}

export function setupFieldScene(app: Application, source: FieldSource): Teardown {
  const graphics = new Graphics();
  const shotLayer = new Graphics();
  const labels = new Container();
  app.stage.addChild(graphics, shotLayer, labels);
  app.stage.eventMode = 'static';
  app.stage.cursor = 'default';

  let state = source.getState();
  let viewport: Viewport | undefined;
  let hoveredId: string | undefined;
  let playback: { shotId: string; startedAt: number; durationMs: number } | undefined;
  let seenShotId: string | undefined;

  const progress = () => (playback ? shotProgress(playback.startedAt, performance.now(), playback.durationMs) : 1);

  /** Новый live-выстрел запускает анимацию один раз (повтор shotId не перезапускает); смена или исчезновение выстрела отменяет старую. */
  function syncShot() {
    const shot = state?.shot;
    if (!shot) {
      playback = undefined;
      seenShotId = undefined;
      return;
    }
    if (playback && playback.shotId !== shot.shotId) playback = undefined;
    if (shot.shotId === seenShotId) return;
    seenShotId = shot.shotId;
    if (shot.live) {
      playback = { shotId: shot.shotId, startedAt: performance.now(), durationMs: shotDurationMs(shot.points) };
    }
  }

  function drawShot() {
    shotLayer.clear();
    const shot = state?.shot;
    if (!shot || !viewport) return;
    const p = progress();
    const animating = playback !== undefined && p < 1;
    const path = visiblePath(shot.points, animating ? p : 1).map((point) => viewport!.worldToScreen(point));
    const head = path[path.length - 1];
    if (!head) return;
    const alpha = animating ? 1 : 0.4;
    if (path.length > 1) {
      shotLayer.moveTo(path[0]!.x, path[0]!.y);
      for (const point of path.slice(1)) shotLayer.lineTo(point.x, point.y);
      shotLayer.stroke({ width: 3, color: COLORS.shot, alpha, join: 'round', cap: 'round' });
    }
    if (animating) {
      shotLayer.circle(head.x, head.y, 5).fill(COLORS.shot);
    } else if (shot.stopReason === 'hit') {
      shotLayer.circle(head.x, head.y, 9).stroke({ width: 3, color: COLORS.impact, alpha });
    }
  }

  const tick = () => {
    if (!playback || progress() >= 1) {
      app.ticker.remove(tick);
      playback = undefined;
      draw();
      return;
    }
    drawShot();
  };

  function draw() {
    state = source.getState();
    syncShot();
    if (playback) {
      app.ticker.remove(tick);
      app.ticker.add(tick);
    }
    app.stage.hitArea = app.screen;
    graphics.clear();
    shotLayer.clear();
    for (const child of labels.removeChildren()) child.destroy();
    if (!state) {
      viewport = undefined;
      return;
    }
    viewport = createViewport({
      worldWidth: state.width,
      worldHeight: state.height,
      screenWidth: app.screen.width,
      screenHeight: app.screen.height,
      padding: PADDING,
    });

    drawField(graphics, viewport, state);
    for (const obstacle of state.obstacles) {
      const center = viewport.worldToScreen(obstacle);
      graphics
        .circle(center.x, center.y, obstacle.r * viewport.scale)
        .fill(COLORS.obstacle)
        .stroke({ width: 2, color: COLORS.obstacleEdge });
    }
    for (const fighter of state.fighters) {
      // Поражение показывается в момент конца анимации, а не в момент прихода события.
      const hiddenKill = playback !== undefined && fighter.id === state.shot?.destroyedFighterId;
      drawFighter(graphics, labels, viewport, hiddenKill ? { ...fighter, alive: true } : fighter, {
        selected: fighter.id === state.selectedFighterId,
        hovered: fighter.id === hoveredId,
      });
    }
    drawShot();
  }

  function fighterAt(event: FederatedPointerEvent): Fighter | undefined {
    if (!state || !viewport) return undefined;
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
    if (fighter) source.onFighterClick?.(fighter);
  };

  app.stage.on('pointermove', onMove);
  app.stage.on('pointerleave', onLeave);
  app.stage.on('pointertap', onTap);
  app.renderer.on('resize', draw);
  const unsubscribe = source.subscribe(draw);
  draw();

  return () => {
    unsubscribe();
    app.ticker.remove(tick);
    playback = undefined;
    app.renderer.off('resize', draw);
    app.stage.off('pointermove', onMove);
    app.stage.off('pointerleave', onLeave);
    app.stage.off('pointertap', onTap);
    graphics.destroy();
    shotLayer.destroy();
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

export function mountFieldScene(source: FieldSource = staticFieldSource(sampleField)) {
  return (host: HTMLElement, onError: (error: unknown) => void): SceneHandle =>
    mountScene(host, {
      createApp: () => new Application(),
      setup: (app) => setupFieldScene(app, source),
      onError,
    });
}
