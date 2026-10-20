import { z } from 'zod';
import { browserTabSession, type TabSession } from './tabSession';

const guestSchema = z.object({ guestId: z.string(), token: z.string().optional() });
const roomSchema = z.object({ roomId: z.string(), inviteId: z.string() });

/** Сервер в ответе возвращает только guestId; имя остаётся тем, что ввёл игрок. */
export interface Guest {
  guestId: string;
  displayName: string;
}
export type Room = z.infer<typeof roomSchema>;

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string | undefined,
  ) {
    super(`API ${status}${code ? ` ${code}` : ''}`);
  }
}

export type Fetch = typeof fetch;

export interface RoomsApi {
  createGuest(displayName: string): Promise<Guest>;
  createRoom(): Promise<Room>;
  joinRoom(roomId: string, inviteId: string): Promise<void>;
  setReady(roomId: string, ready: boolean): Promise<void>;
  leaveRoom(roomId: string): Promise<void>;
}

/** REST из docs/contract.md §8. Личность — токен вкладки (Authorization: Bearer, см. tabSession.ts) с запасной HttpOnly-cookie; guestId из ответа в запросах не используется. */
export function createRoomsApi(
  fetchImpl: Fetch = (...args) => fetch(...args),
  session: TabSession = browserTabSession(),
): RoomsApi {
  async function call(method: string, path: string, body?: unknown, extra: Record<string, string> = {}): Promise<unknown> {
    const headers: Record<string, string> = { ...extra };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    const token = session.get();
    if (token) headers.Authorization = `Bearer ${token}`;
    const response = await fetchImpl(path, {
      method,
      credentials: 'include',
      headers: Object.keys(headers).length ? headers : undefined,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const text = await response.text();
    let json: unknown;
    try {
      json = text ? JSON.parse(text) : undefined;
    } catch {
      json = undefined;
    }
    if (!response.ok) {
      const code = (json as { code?: unknown } | undefined)?.code;
      throw new ApiError(response.status, typeof code === 'string' ? code : undefined);
    }
    return json;
  }

  const room = (id: string) => `/api/rooms/${encodeURIComponent(id)}`;

  return {
    async createGuest(displayName) {
      const { guestId, token } = guestSchema.parse(await call('POST', '/api/guest', { displayName }, { 'X-Session-Mode': 'tab' }));
      if (token) session.set(token);
      return { guestId, displayName };
    },
    async createRoom() {
      return roomSchema.parse(await call('POST', '/api/rooms'));
    },
    async joinRoom(roomId, inviteId) {
      await call('POST', `${room(roomId)}/join`, { inviteId });
    },
    async setReady(roomId, ready) {
      await call('POST', `${room(roomId)}/ready`, { ready });
    },
    async leaveRoom(roomId) {
      await call('DELETE', room(roomId));
    },
  };
}

export function inviteLink(origin: string, room: Room): string {
  const url = new URL(origin);
  url.search = new URLSearchParams({ room: room.roomId, invite: room.inviteId }).toString();
  return url.toString();
}

export function readInvite(search: string): { roomId: string; inviteId: string } | undefined {
  const params = new URLSearchParams(search);
  const roomId = params.get('room');
  const inviteId = params.get('invite');
  return roomId && inviteId ? { roomId, inviteId } : undefined;
}
