import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { initialGameState, type GameState } from '../state/gameState';
import { ConnectionBanner } from './ConnectionBanner';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const containers: HTMLElement[] = [];

function render(state: GameState, onRetry = vi.fn()) {
  const container = document.createElement('div');
  document.body.append(container);
  containers.push(container);
  act(() => createRoot(container).render(<ConnectionBanner state={state} onRetry={onRetry} />));
  return { container, onRetry };
}

afterEach(() => containers.splice(0).forEach((c) => c.remove()));

describe('ConnectionBanner', () => {
  it('молчит при нормальном соединении', () => {
    expect(render({ ...initialGameState, phase: 'active', connection: 'open' }).container.textContent).toBe('');
  });

  it('при переподключении сообщает, что выстрелы не отправляются', () => {
    const { container } = render({ ...initialGameState, phase: 'active', connection: 'reconnecting' });
    expect(container.textContent).toContain('переподключаемся');
  });

  it('после сдачи авто-попыток даёт кнопку «Повторить»', () => {
    const { container, onRetry } = render({ ...initialGameState, phase: 'active', connection: 'closed' });
    act(() => container.querySelector('button')?.click());
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('после конца матча закрытое соединение — не ошибка', () => {
    expect(render({ ...initialGameState, phase: 'finished', connection: 'closed' }).container.textContent).toBe('');
  });
});
