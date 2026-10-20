import { Application, Graphics, type Ticker } from 'pixi.js';
import { mountScene, type SceneHandle, type Teardown } from './mountScene';

export function setupDemoScene(app: Application): Teardown {
  const square = new Graphics().rect(-40, -40, 80, 80).fill(0x4f8cff);
  app.stage.addChild(square);

  const center = () => square.position.set(app.screen.width / 2, app.screen.height / 2);
  const spin = (ticker: Ticker) => {
    square.rotation += 0.02 * ticker.deltaTime;
  };

  center();
  app.renderer.on('resize', center);
  app.ticker.add(spin);

  return () => {
    app.ticker.remove(spin);
    app.renderer.off('resize', center);
    square.destroy();
  };
}

export function mountDemoScene(host: HTMLElement, onError: (error: unknown) => void): SceneHandle {
  return mountScene(host, {
    createApp: () => new Application(),
    setup: setupDemoScene,
    onError,
  });
}
