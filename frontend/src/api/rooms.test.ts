import { describe, expect, it, vi } from 'vitest';
import { ApiError, createRoomsApi, inviteLink, readInvite } from './rooms';
import { memoryTabSession, wsProtocols } from './tabSession';

const json = (status: number, body?: unknown) => new Response(body === undefined ? null : JSON.stringify(body), { status });

describe('токен вкладки', () => {
  it('createGuest запрашивает tab-режим и запоминает токен; дальше он идёт в Authorization и в subprotocol WS', async () => {
    const session = memoryTabSession();
    const fetchMock = vi.fn(async (url: RequestInfo | URL, _init?: RequestInit) =>
      url === '/api/guest' ? json(201, { guestId: 'g1', token: 'tok' }) : json(201, { roomId: 'r', inviteId: 'i' }),
    );
    const api = createRoomsApi(fetchMock as unknown as typeof fetch, session);

    await api.createGuest('Аня');
    expect((fetchMock.mock.calls[0]![1]!.headers as Record<string, string>)['X-Session-Mode']).toBe('tab');
    expect(session.get()).toBe('tok');

    await api.createRoom();
    expect((fetchMock.mock.calls[1]![1]!.headers as Record<string, string>).Authorization).toBe('Bearer tok');
    expect(wsProtocols(session)).toEqual(['graphwar.v1', 'bearer.tok']);
    expect(wsProtocols(memoryTabSession())).toBeUndefined();
  });
});

describe('createRoomsApi', () => {
  it('посылает запросы по контракту с cookie', async () => {
    const fetchMock = vi.fn(async (_url: RequestInfo | URL, _init?: RequestInit) => json(201, { guestId: 'g1' }));
    const api = createRoomsApi(fetchMock as unknown as typeof fetch);

    await expect(api.createGuest('Аня')).resolves.toEqual({ guestId: 'g1', displayName: 'Аня' });
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/guest');
    expect(init).toMatchObject({ method: 'POST', credentials: 'include', body: '{"displayName":"Аня"}' });
  });

  it('join/ready/leave идут на нужные адреса, идентификаторы экранируются', async () => {
    const fetchMock = vi.fn(async (_url: RequestInfo | URL, _init?: RequestInit) => json(204));
    const api = createRoomsApi(fetchMock as unknown as typeof fetch);

    await api.joinRoom('r 1', 'inv');
    await api.setReady('r 1', true);
    await api.leaveRoom('r 1');

    expect(fetchMock.mock.calls.map(([url, init]) => [init?.method, url])).toEqual([
      ['POST', '/api/rooms/r%201/join'],
      ['POST', '/api/rooms/r%201/ready'],
      ['DELETE', '/api/rooms/r%201'],
    ]);
    expect(fetchMock.mock.calls[0]![1]!.body).toBe('{"inviteId":"inv"}');
    expect(fetchMock.mock.calls[1]![1]!.body).toBe('{"ready":true}');
  });

  it('ошибка HTTP превращается в ApiError с кодом сервера', async () => {
    const api = createRoomsApi((async () => json(409, { code: 'room_full' })) as unknown as typeof fetch);
    await expect(api.joinRoom('r', 'i')).rejects.toMatchObject({ status: 409, code: 'room_full' });
    await expect(api.joinRoom('r', 'i')).rejects.toBeInstanceOf(ApiError);
  });

  it('ответ неверной формы отвергается, а не принимается молча', async () => {
    const api = createRoomsApi((async () => json(200, { roomId: 1 })) as unknown as typeof fetch);
    await expect(api.createRoom()).rejects.toThrow();
  });
});

describe('приглашение', () => {
  it('ссылка собирается и читается обратно', () => {
    const link = inviteLink('http://127.0.0.1:5173/', { roomId: 'r1', inviteId: 'i 1' });
    expect(readInvite(new URL(link).search)).toEqual({ roomId: 'r1', inviteId: 'i 1' });
  });

  it('без обоих параметров приглашения нет', () => {
    expect(readInvite('?room=r1')).toBeUndefined();
    expect(readInvite('')).toBeUndefined();
  });
});
