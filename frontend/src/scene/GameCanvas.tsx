import { useEffect, useRef, useState } from 'react';
import { mountDemoScene } from './demoScene';
import type { SceneHandle } from './mountScene';

export type MountScene = (host: HTMLElement, onError: (error: unknown) => void) => SceneHandle;

interface GameCanvasProps {
  mount?: MountScene;
}

export function GameCanvas({ mount = mountDemoScene }: GameCanvasProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    setFailed(false);
    const scene = mount(host, (error) => {
      console.error('Не удалось запустить игровую сцену', error);
      setFailed(true);
    });
    return () => scene.dispose();
  }, [mount]);

  return (
    <div className="game-canvas" ref={hostRef} data-testid="game-canvas">
      {failed && <p role="alert">Не удалось запустить графику. Обновите страницу или откройте другой браузер.</p>}
    </div>
  );
}
