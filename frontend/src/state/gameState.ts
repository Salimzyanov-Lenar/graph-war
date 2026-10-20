import { parseServerMessage, type ParseResult } from '../contract/parse';
import type { MatchBody, MatchResult, Phase, Player, ServerMessage, Shot, Turn } from '../contract/schemas';
import type { FieldState } from '../field/types';

export type ConnectionStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'resyncing';

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

export type IgnoredReason = 'no_stream' | 'other_stream' | 'resyncing' | 'duplicate' | 'out_of_phase';

export interface Outcome {
  state: GameState;
  /** true — после этого сообщения клиент должен запросить у сервера новый snapshot. */
  resync: boolean;
  ignored?: IgnoredReason;
  rejected?: Extract<ParseResult, { ok: false }>;
}

const keep = (state: GameState, ignored: IgnoredReason): Outcome => ({ state, resync: false, ignored });

export function setConnection(state: GameState, connection: ConnectionStatus): GameState {
  return state.connection === connection ? state : { ...state, connection };
}

/** Смена или выход из комнаты: игровое состояние сбрасывается, статус соединения остаётся. */
export function leaveRoom(state: GameState): GameState {
  return { ...initialGameState, connection: state.connection };
}

export function applyRawMessage(state: GameState, raw: unknown): Outcome {
  const parsed = parseServerMessage(raw);
  if (!parsed.ok) return { state, resync: false, rejected: parsed };
  return applyServerMessage(state, parsed.message);
}

export function applyServerMessage(state: GameState, message: ServerMessage): Outcome {
  if (message.type === 'snapshot') {
    const { payload } = message;
    return {
      resync: false,
      state: {
        connection: state.connection === 'resyncing' ? 'open' : state.connection,
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

  if (state.streamId === undefined) return keep(state, 'no_stream');
  if (message.streamId !== state.streamId) return keep(state, 'other_stream');

  // Ошибки команд могут не входить в последовательность событий матча, поэтому seq для них не проверяется.
  if (message.type === 'error') {
    return { resync: false, state: { ...state, lastError: message.payload } };
  }

  if (state.connection === 'resyncing') return keep(state, 'resyncing');
  if (message.seq <= state.lastSeq) return keep(state, 'duplicate');
  if (message.seq > state.lastSeq + 1) {
    return { resync: true, state: { ...state, connection: 'resyncing' } };
  }

  const next = applyEvent({ ...state, lastSeq: message.seq }, message);
  return next ? { resync: false, state: next } : keep(state, 'out_of_phase');
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
