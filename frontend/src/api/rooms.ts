import { z } from 'zod';

const guestSchema = z.object({ guestId: z.string(), displayName: z.string() });
const roomSchema = z.object({ roomId: z.string(), inviteId: z.string() });

export type Guest = z.infer<typeof guestSchema>;
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

/** REST из docs/contract.md §8. Личность — HttpOnly-cookie, поэтому всегда credentials: 'include'; guestId из ответа в запросах не используется. */
export function createRoomsApi(fetchImpl: Fetch = (...args) => fetch(...args)): RoomsApi {
  async function call(method: string, path: string, body?: unknown): Promise<unknown> {
    const response = await fetchImpl(path, {
      method,
      credentials: 'include',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
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
      return guestSchema.parse(await call('POST', '/api/guest', { displayName }));
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
