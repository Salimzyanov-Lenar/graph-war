import { describe, expect, it, vi } from 'vitest';
import { mountScene, type SceneApp } from './mountScene';

class FakeApp implements SceneApp {
  readonly canvas = document.createElement('canvas');
  destroyCalls = 0;
  private resolveInit!: () => void;
  private rejectInit!: (error: unknown) => void;
  private readonly initPromise = new Promise<void>((resolve, reject) => {
    this.resolveInit = resolve;
    this.rejectInit = reject;
  });

  init(): Promise<void> {
    return this.initPromise;
  }

  finishInit() {
    this.resolveInit();
  }

  failInit(error: unknown) {
    this.rejectInit(error);
  }

  destroy(rendererDestroyOptions?: { removeView?: boolean }) {
    this.destroyCalls += 1;
    if (rendererDestroyOptions?.removeView) this.canvas.remove();
  }
}

function setup() {
  const host = document.createElement('div');
  const app = new FakeApp();
  const teardown = vi.fn();
  const setupScene = vi.fn(() => teardown);
  const onError = vi.fn();
  const handle = mountScene(host, { createApp: () => app, setup: setupScene, onError });
  return { host, app, teardown, setupScene, onError, handle };
}

describe('mountScene', () => {
  it('добавляет один canvas после init и строит сцену', async () => {
    const { host, app, setupScene, handle } = setup();
    expect(host.querySelectorAll('canvas')).toHaveLength(0);

    app.finishInit();
    await handle.ready;

    expect(host.querySelectorAll('canvas')).toHaveLength(1);
    expect(setupScene).toHaveBeenCalledTimes(1);
  });

  it('dispose после init снимает сцену, уничтожает приложение и убирает canvas', async () => {
    const { host, app, teardown, handle } = setup();
    app.finishInit();
    await handle.ready;

    handle.dispose();
    handle.dispose();

    expect(teardown).toHaveBeenCalledTimes(1);
    expect(app.destroyCalls).toBe(1);
    expect(host.querySelectorAll('canvas')).toHaveLength(0);
  });

  it('dispose до завершения init не добавляет canvas и уничтожает приложение после init', async () => {
    const { host, app, setupScene, handle } = setup();

    handle.dispose();
    expect(app.destroyCalls).toBe(0);

    app.finishInit();
    await handle.ready;

    expect(host.querySelectorAll('canvas')).toHaveLength(0);
    expect(setupScene).not.toHaveBeenCalled();
    expect(app.destroyCalls).toBe(1);
  });

  it('ошибка init сообщается один раз и не оставляет canvas', async () => {
    const { host, app, onError, handle } = setup();
    const failure = new Error('webgl недоступен');

    app.failInit(failure);
    await handle.ready;
    handle.dispose();

    expect(onError).toHaveBeenCalledExactlyOnceWith(failure);
    expect(host.querySelectorAll('canvas')).toHaveLength(0);
    expect(app.destroyCalls).toBe(0);
  });

  it('ошибка init после dispose не сообщается', async () => {
    const { app, onError, handle } = setup();

    handle.dispose();
    app.failInit(new Error('поздно'));
    await handle.ready;

    expect(onError).not.toHaveBeenCalled();
  });

  it('падение setup уничтожает приложение и сообщает об ошибке', async () => {
    const host = document.createElement('div');
    const app = new FakeApp();
    const failure = new Error('сломанная сцена');
    const onError = vi.fn();
    const handle = mountScene(host, {
      createApp: () => app,
      setup: () => {
        throw failure;
      },
      onError,
    });

    app.finishInit();
    await handle.ready;
    handle.dispose();

    expect(onError).toHaveBeenCalledExactlyOnceWith(failure);
    expect(app.destroyCalls).toBe(1);
    expect(host.querySelectorAll('canvas')).toHaveLength(0);
  });
});
