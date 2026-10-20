import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fixtures } from '../contract/fixtures';
import type { ClientCommand } from '../contract/schemas';
import { applyServerMessage, initialGameState, type GameState } from '../state/gameState';
import { MatchHud } from './MatchHud';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const active = (): GameState => applyServerMessage(initialGameState, fixtures.activeSnapshot).state;

describe('MatchHud', () => {
  let container: HTMLElement;
  let root: Root;

  beforeEach(() => {
    vi.useFakeTimers();
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.useRealTimers();
  });

  const render = (state: GameState, send: (c: ClientCommand) => void = () => {}) => act(() => root.render(<MatchHud state={state} send={send} />));
  const button = (text: string) => [...container.querySelectorAll('button')].find((b) => b.textContent?.includes(text));

  it('показывает счёт живых и чей ход', () => {
    render(active());
    expect(container.textContent).toContain('3');
    expect(container.querySelector('.hud-turn')?.textContent).toMatch(/Ваш ход|Ход соперника/);
  });

  it('отсчёт идёт по серверному времени и не уходит ниже нуля без решения сервера', () => {
    const state = active();
    const turn = state.turn!;
    render({ ...state, turn: { ...turn, serverTime: 1_000_000, deadlineAt: 1_000_000 + 3000 } });
    expect(container.querySelector('.hud-clock')?.textContent).toBe('0:03');

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(container.querySelector('.hud-clock')?.textContent).toBe('0:00');
    expect(container.textContent).toContain('ждём решения сервера');
  });

  it('сдача требует подтверждения и отправляет confirm: true один раз', () => {
    const send = vi.fn();
    render(active(), send);

    act(() => button('Сдаться')!.click());
    expect(send).not.toHaveBeenCalled();
    act(() => button('Отмена')!.click());
    expect(button('Да, сдаться')).toBeUndefined();

    act(() => button('Сдаться')!.click());
    act(() => button('Да, сдаться')!.click());
    expect(send).toHaveBeenCalledExactlyOnceWith({ type: 'match.surrender', payload: { confirm: true } });
    expect(button('Сдача отправлена')?.disabled).toBe(true);
  });

  it('после итога показывает исход и убирает действия', () => {
    const state = active();
    render({ ...state, phase: 'finished', result: { winnerSide: state.match!.yourSide, reason: 'team_wiped' } });
    expect(container.querySelector('.hud-outcome')?.textContent).toContain('Победа');
    expect(button('Сдаться')).toBeUndefined();
    expect(container.querySelector('.hud-clock')).toBeNull();
  });
});
