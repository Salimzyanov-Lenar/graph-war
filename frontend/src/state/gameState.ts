import { parseServerMessage, type ParseResult } from '../contract/parse';
import type { MatchBody, MatchResult, Phase, Player, ServerMessage, Shot, Turn } from '../contract/schemas';
import type { FieldState } from '../field/types';

export type ConnectionStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'reconnecting';

export interface GameState {
  connection: ConnectionStatus;
  streamId?: string;
  lastSeq: number;
  phase: 'idle' | Phase;
  players: readonly Player[];
  match?: MatchBody;
  turn?: Turn;
  /** Последний принятый выстрел; пока идёт `resolving`, рисуется анимацией. Убитый боец в `match.fighters` помечен сразу. */
  lastShot?: Shot;
  result?: MatchResult;
  lastError?: { code: string; message: string };
}

export const initialGameState: GameState = {
  connection: 'idle',
  lastSeq: 0,
  phase: 'idle',
  players: [],
};

export type IgnoredReason = 'no_stream' | 'other_stream' | 'reconnecting' | 'duplicate' | 'out_of_phase';

export interface Outcome {
  state: GameState;
  /** true — в потоке дыра: клиент переподключается к /ws, первым сообщением придёт свежий snapshot. */
  reconnect: boolean;
  ignored?: IgnoredReason;
  rejected?: Extract<ParseResult, { ok: false }>;
}

const keep = (state: GameState, ignored: IgnoredReason): Outcome => ({ state, reconnect: false, ignored });

export function setConnection(state: GameState, connection: ConnectionStatus): GameState {
  return state.connection === connection ? state : { ...state, connection };
}

/** Смена или выход из комнаты: игровое состояние сбрасывается, статус соединения остаётся. */
export function leaveRoom(state: GameState): GameState {
  return { ...initialGameState, connection: state.connection };
}

export function applyRawMessage(state: GameState, raw: unknown): Outcome {
  const parsed = parseServerMessage(raw);
  if (!parsed.ok) return { state, reconnect: false, rejected: parsed };
  return applyServerMessage(state, parsed.message);
}

export function applyServerMessage(state: GameState, message: ServerMessage): Outcome {
  if (message.type === 'snapshot') {
    const { payload } = message;
    return {
      reconnect: false,
      state: {
        connection: state.connection === 'reconnecting' ? 'open' : state.connection,
        streamId: message.streamId,
        lastSeq: message.seq,
        phase: payload.phase,
        players: payload.players ?? [],
        match: payload.match,
        turn: payload.turn,
        lastShot: payload.lastShot,
        result: payload.result,
        lastError: undefined,
      },
    };
  }

  // Ошибка команды приходит только отправителю, без streamId и seq: в поток событий не входит.
  if (message.type === 'error') {
    return { reconnect: false, state: { ...state, lastError: message.payload } };
  }

  if (state.streamId === undefined) return keep(state, 'no_stream');
  if (message.streamId !== state.streamId) return keep(state, 'other_stream');

  if (state.connection === 'reconnecting') return keep(state, 'reconnecting');
  if (message.seq <= state.lastSeq) return keep(state, 'duplicate');
  if (message.seq > state.lastSeq + 1) {
    return { reconnect: true, state: { ...state, connection: 'reconnecting' } };
  }

  const next = applyEvent({ ...state, lastSeq: message.seq }, message);
  return next ? { reconnect: false, state: next } : keep(state, 'out_of_phase');
}

function applyEvent(state: GameState, message: Exclude<ServerMessage, { type: 'snapshot' | 'error' }>): GameState | undefined {
  switch (message.type) {
    case 'room.changed':
      return { ...state, players: message.payload.players, phase: state.phase === 'idle' ? 'lobby' : state.phase };

    case 'match.started':
      return { ...state, phase: 'active', match: message.payload, turn: undefined, lastShot: undefined, result: undefined, lastError: undefined };

    case 'turn.opened':
      if (state.phase !== 'active' && state.phase !== 'resolving') return undefined;
      return { ...state, phase: 'active', turn: message.payload, lastError: undefined };

    case 'shot.resolved': {
      if ((state.phase !== 'active' && state.phase !== 'resolving') || !state.match) return undefined;
      const destroyed = message.payload.destroyedFighterId;
      const fighters = destroyed
        ? state.match.fighters.map((f) => (f.id === destroyed ? { ...f, alive: false } : f))
        : state.match.fighters;
      return { ...state, phase: 'resolving', lastShot: message.payload, lastError: undefined, match: { ...state.match, fighters } };
    }

    case 'match.finished':
      return { ...state, phase: 'finished', turn: undefined, result: message.payload };
  }
}

/** Геометрия для сцены поля берётся только из серверного состояния. */
export function toFieldState(state: GameState, selectedFighterId?: string): FieldState | undefined {
  if (!state.match) return undefined;
  const { map, fighters } = state.match;
  return { width: map.width, height: map.height, obstacles: map.obstacles, fighters, selectedFighterId };
}
