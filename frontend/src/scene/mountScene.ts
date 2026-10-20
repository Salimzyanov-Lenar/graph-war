export type Teardown = () => void;

export interface SceneInitOptions {
  resizeTo: HTMLElement;
  background: number;
  antialias: boolean;
  resolution: number;
  autoDensity: boolean;
}

export interface SceneApp {
  readonly canvas: HTMLCanvasElement;
  init(options: SceneInitOptions): Promise<void>;
  destroy(rendererDestroyOptions?: { removeView?: boolean }, options?: { children?: boolean }): void;
}

export interface SceneOptions<A extends SceneApp> {
  createApp: () => A;
  setup: (app: A) => Teardown;
  onError?: (error: unknown) => void;
}

export interface SceneHandle {
  dispose(): void;
  ready: Promise<void>;
}

type State = 'initializing' | 'ready' | 'failed' | 'disposed';

const BACKGROUND = 0x10151f;

export function mountScene<A extends SceneApp>(
  host: HTMLElement,
  { createApp, setup, onError }: SceneOptions<A>,
): SceneHandle {
  const app = createApp();
  let state: State = 'initializing';
  let teardown: Teardown | undefined;
  let canvas: HTMLCanvasElement | undefined;

  const destroyApp = () => {
    app.destroy({ removeView: true }, { children: true });
    canvas?.remove();
  };

  const ready = app
    .init({
      resizeTo: host,
      background: BACKGROUND,
      antialias: true,
      resolution: window.devicePixelRatio || 1,
      autoDensity: true,
    })
    .then(
      () => {
        if (state === 'disposed') {
          destroyApp();
          return;
        }
        canvas = app.canvas;
        host.appendChild(canvas);
        try {
          teardown = setup(app);
          state = 'ready';
        } catch (error) {
          state = 'failed';
          destroyApp();
          onError?.(error);
        }
      },
      (error: unknown) => {
        if (state === 'disposed') return;
        state = 'failed';
        onError?.(error);
      },
    );

  return {
    ready,
    dispose() {
      if (state === 'disposed') return;
      const previous = state;
      state = 'disposed';
      if (previous === 'ready') {
        teardown?.();
        teardown = undefined;
        destroyApp();
      }
    },
  };
}
