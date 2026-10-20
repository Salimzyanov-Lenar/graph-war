import { StrictMode, act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { GameCanvas, type MountScene } from './GameCanvas';
import { mountScene, type SceneApp } from './mountScene';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

class FakeApp implements SceneApp {
  readonly canvas = document.createElement('canvas');
  destroyed = false;

  init(): Promise<void> {
    return Promise.resolve();
  }

  destroy(rendererDestroyOptions?: { removeView?: boolean }) {
    this.destroyed = true;
    if (rendererDestroyOptions?.removeView) this.canvas.remove();
  }
}

function createHarness() {
  const apps: FakeApp[] = [];
  const teardowns = vi.fn();
  const mount: MountScene = (host, onError) =>
    mountScene(host, {
      createApp: () => {
        const app = new FakeApp();
        apps.push(app);
        return app;
      },
      setup: () => teardowns,
      onError,
    });
  return { apps, teardowns, mount };
}

describe('GameCanvas в React.StrictMode', () => {
  let container: HTMLElement;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.append(container);
  });

  afterEach(() => {
    container.remove();
  });

  it('оставляет ровно один живой canvas после двойного эффекта и снимает его при unmount', async () => {
    const { apps, teardowns, mount } = createHarness();
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <StrictMode>
          <GameCanvas mount={mount} />
        </StrictMode>,
      );
    });

    expect(container.querySelectorAll('canvas')).toHaveLength(1);
    expect(apps.filter((app) => !app.destroyed)).toHaveLength(1);
    expect(apps.filter((app) => app.destroyed)).toHaveLength(apps.length - 1);

    await act(async () => {
      root.unmount();
    });

    expect(container.querySelectorAll('canvas')).toHaveLength(0);
    expect(apps.every((app) => app.destroyed)).toBe(true);
    expect(teardowns).toHaveBeenCalledTimes(1);
  });

  it('повторные mount/unmount не накапливают canvas и приложения', async () => {
    const { apps, mount } = createHarness();

    for (let i = 0; i < 5; i += 1) {
      const root = createRoot(container);
      await act(async () => {
        root.render(
          <StrictMode>
            <GameCanvas mount={mount} />
          </StrictMode>,
        );
      });
      expect(container.querySelectorAll('canvas')).toHaveLength(1);
      await act(async () => {
        root.unmount();
      });
      expect(container.querySelectorAll('canvas')).toHaveLength(0);
    }

    expect(apps.every((app) => app.destroyed)).toBe(true);
  });

  it('unmount сразу после render, до завершения init, не оставляет canvas', async () => {
    const { apps, mount } = createHarness();
    const root = createRoot(container);

    act(() => {
      root.render(<GameCanvas mount={mount} />);
    });
    act(() => {
      root.unmount();
    });
    await act(async () => {
      await Promise.resolve();
    });

    expect(container.querySelectorAll('canvas')).toHaveLength(0);
    expect(apps.length).toBeGreaterThan(0);
    expect(apps.every((app) => app.destroyed)).toBe(true);
  });

  it('показывает сообщение об ошибке вместо белого экрана при сбое init', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    const mount: MountScene = (host, onError) =>
      mountScene(host, {
        createApp: () => ({
          canvas: document.createElement('canvas'),
          init: () => Promise.reject(new Error('нет WebGL')),
          destroy: () => {},
        }),
        setup: () => () => {},
        onError,
      });
    const root = createRoot(container);

    await act(async () => {
      root.render(<GameCanvas mount={mount} />);
    });

    expect(container.querySelector('[role="alert"]')).not.toBeNull();
    expect(container.querySelectorAll('canvas')).toHaveLength(0);

    await act(async () => {
      root.unmount();
    });
    consoleError.mockRestore();
  });
});
