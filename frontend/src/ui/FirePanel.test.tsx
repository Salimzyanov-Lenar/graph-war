import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fixtures } from '../contract/fixtures';
import type { ClientCommand } from '../contract/schemas';
import { applyRawMessage, initialGameState, type GameState } from '../state/gameState';
import { FirePanel } from './FirePanel';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const apply = (state: GameState, raw: unknown): GameState => applyRawMessage(state, raw).state;
const active = (): GameState => apply(initialGameState, fixtures.activeSnapshot);

describe('FirePanel', () => {
  let container: HTMLElement;
  let root: Root;
  let sent: ClientCommand[];
  let ids: number;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    sent = [];
    ids = 0;
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  function render(state: GameState, selectedId: string | undefined = 'a1', onSelect: (id: string) => void = () => {}) {
    act(() => {
      root.render(
        <FirePanel
          state={state}
          selectedId={selectedId}
          onSelect={onSelect}
          send={(command) => sent.push(command)}
          newRequestId={() => `req-${(ids += 1)}`}
        />,
      );
    });
  }

  const input = () => container.querySelector('input') as HTMLInputElement;
  const submitButton = () => container.querySelector('button[type="submit"]') as HTMLButtonElement;
  const form = () => container.querySelector('form') as HTMLFormElement;

  function type(value: string) {
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
      setter.call(input(), value);
      input().dispatchEvent(new Event('input', { bubbles: true }));
    });
  }
  const submit = () => act(() => form().dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));

  it('показывает только своих бойцов; выбор доступен только для живых', () => {
    const onSelect = vi.fn();
    const withDead = apply(active(), {
      type: 'shot.resolved',
      streamId: 'match:m1',
      seq: 11,
      payload: { shotId: 's1', turnId: 't1', points: [[1, 1]], stopReason: 'hit', destroyedFighterId: 'a2' },
    });
    render(withDead, 'a1', onSelect);

    const buttons = [...container.querySelectorAll<HTMLButtonElement>('.fighter-list button')];
    expect(buttons.map((b) => b.textContent)).toEqual(['A1 — жив', 'A2 — уничтожен', 'A3 — жив']);
    expect(buttons.map((b) => b.disabled)).toEqual([false, true, false]);
    expect(buttons[0]?.getAttribute('aria-pressed')).toBe('true');

    act(() => buttons[0]?.click());
    expect(onSelect).toHaveBeenCalledWith('a1');
  });

  it('отправляет match.fire по Enter и не даёт повторить до подтверждения состоянием', () => {
    const state = active();
    render(state);
    type(' 5*sin(x/3) ');
    submit();

    expect(sent).toEqual([
      {
        type: 'match.fire',
        payload: { matchId: 'm1', expectedTurnId: 't1', fighterId: 'a1', expression: '5*sin(x/3)', requestId: 'req-1' },
      },
    ]);
    expect(submitButton().disabled).toBe(true);

    submit();
    submit();
    expect(sent).toHaveLength(1);

    render(apply(state, fixtures.shotResolved));
    expect(sent).toHaveLength(1);
  });

  it('ошибка поля не стирает текст и разблокирует повторную отправку', () => {
    const state = active();
    render(state);
    type('2x');
    submit();
    expect(submitButton().disabled).toBe(true);

    const failed = apply(state, {
      type: 'error',
      streamId: 'match:m1',
      seq: 0,
      payload: { code: 'bad_expression', message: 'неверная формула' },
    });
    render(failed);

    expect(input().value).toBe('2x');
    expect(container.querySelector('[role="alert"]')).not.toBeNull();
    expect(submitButton().disabled).toBe(false);

    type('2*x');
    submit();
    expect(sent).toHaveLength(2);
    expect(sent[1]).toMatchObject({ payload: { expression: '2*x', requestId: 'req-2' } });
  });

  it('блокирует выстрел на чужом ходу, при пустом поле, без бойца, во время разрешения и после матча', () => {
    const base = active();
    const theirs = apply(base, fixtures.turnOpened);
    const resolving = apply(base, fixtures.shotResolved);
    const finished = apply(resolving, fixtures.matchFinished);

    render(theirs);
    type('x');
    expect(submitButton().disabled).toBe(true);
    submit();

    render(base, 'none');
    expect(submitButton().disabled).toBe(true);

    render(base, 'a1');
    type('');
    expect(submitButton().disabled).toBe(true);

    render(resolving);
    type('x');
    expect(submitButton().disabled).toBe(true);
    submit();

    render(finished);
    expect(submitButton().disabled).toBe(true);
    submit();

    expect(sent).toHaveLength(0);
  });

  it('чужой и уничтоженный боец не выбираются: отправка заблокирована', () => {
    const state = active();
    render(state, 'b1');
    type('x');
    expect(submitButton().disabled).toBe(true);
    submit();
    expect(sent).toHaveLength(0);
  });

  it('справка содержит функции из контракта, предупреждения и подставляет пример', () => {
    render(active());
    const help = container.querySelector('.formula-help')!;
    for (const fn of ['sin', 'cos', 'sqrt', 'abs', 'ln', 'exp']) expect(help.textContent).toContain(fn);
    expect(help.textContent).toContain('ln(x+1)');

    const example = help.querySelector<HTMLButtonElement>('.example')!;
    act(() => example.click());
    expect(input().value).toBe(example.textContent);
  });
});
