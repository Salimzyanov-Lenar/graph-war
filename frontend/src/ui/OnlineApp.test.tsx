import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type RoomsApi } from '../api/rooms';
import { OnlineApp, apiErrorText } from './OnlineApp';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

class StubSocket {
  static instances: StubSocket[] = [];
  onopen = null;
  onclose = null;
  onmessage = null;
  onerror = null;
  readyState = 0;
  constructor(readonly url: string) {
    StubSocket.instances.push(this);
  }
  send() {}
  close() {}
}

function makeApi(overrides: Partial<RoomsApi> = {}): RoomsApi {
  return {
    createGuest: vi.fn(async (displayName: string) => ({ guestId: 'g1', displayName })),
    createRoom: vi.fn(async () => ({ roomId: 'r1', inviteId: 'inv1' })),
    joinRoom: vi.fn(async () => {}),
    setReady: vi.fn(async () => {}),
    leaveRoom: vi.fn(async () => {}),
    ...overrides,
  };
}

describe('OnlineApp', () => {
  let container: HTMLElement;
  let root: Root;

  beforeEach(() => {
    StubSocket.instances = [];
    vi.stubGlobal('WebSocket', StubSocket);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  });

  const input = () => container.querySelector('input') as HTMLInputElement;
  const button = (text: string) => [...container.querySelectorAll('button')].find((b) => b.textContent?.includes(text)) as HTMLButtonElement;

  function type(value: string) {
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input(), value);
      input().dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  const flush = () => act(async () => {});

  it('имя → создание комнаты → ссылка-приглашение и открытый WebSocket на /ws', async () => {
    const api = makeApi();
    act(() => root.render(<OnlineApp api={api} search="" origin="http://127.0.0.1:5173/" />));

    expect(button('Продолжить').disabled).toBe(true);
    type('  Аня ');
    await act(async () => button('Продолжить').click());
    expect(api.createGuest).toHaveBeenCalledWith('Аня');
    expect(api.joinRoom).not.toHaveBeenCalled();

    await act(async () => button('Создать комнату').click());
    await flush();

    const link = container.querySelector<HTMLInputElement>('input[aria-label="Ссылка-приглашение"]')!;
    expect(link.value).toBe('http://127.0.0.1:5173/?room=r1&invite=inv1');
    expect(StubSocket.instances).toHaveLength(1);
    expect(StubSocket.instances[0]!.url).toMatch(/\/ws$/);
    expect(container.textContent).toContain('Ждём второго игрока');
  });

  it('по ссылке-приглашению гость сразу входит в комнату', async () => {
    const api = makeApi();
    act(() => root.render(<OnlineApp api={api} search="?room=r9&invite=abc" />));

    type('Борис');
    await act(async () => button('Войти в комнату').click());
    await flush();

    expect(api.joinRoom).toHaveBeenCalledWith('r9', 'abc');
    expect(api.createRoom).not.toHaveBeenCalled();
    expect(StubSocket.instances).toHaveLength(1);
  });

  it('ошибка входа показывается и не пускает дальше', async () => {
    const api = makeApi({ joinRoom: vi.fn(async () => Promise.reject(new ApiError(409, 'room_full'))) });
    act(() => root.render(<OnlineApp api={api} search="?room=r9&invite=abc" />));

    type('Борис');
    await act(async () => button('Войти в комнату').click());
    await flush();

    expect(container.querySelector('[role="alert"]')?.textContent).toBe('В комнате уже два игрока.');
    expect(StubSocket.instances).toHaveLength(0);
  });

  it('недоступный сервер объясняется человеческим текстом', () => {
    expect(apiErrorText(new TypeError('Failed to fetch'))).toContain('Не удалось связаться с сервером');
    expect(apiErrorText(new ApiError(500, undefined))).toContain('500');
  });
});
