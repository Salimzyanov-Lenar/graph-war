import { describe, expect, it } from 'vitest';
import { fixtures } from '../contract/fixtures';
import type { ServerMessage } from '../contract/schemas';
import { applyRawMessage, applyServerMessage, initialGameState, leaveRoom, setConnection, toFieldState, type GameState } from './gameState';

const active = (): GameState => applyServerMessage(initialGameState, fixtures.activeSnapshot).state;
type StreamMessage = Exclude<ServerMessage, { type: 'error' }>;
const at = (message: StreamMessage, seq: number): StreamMessage => ({ ...message, seq });

describe('snapshot', () => {
  it('заменяет всё состояние, берёт streamId/seq и переводит в active', () => {
    const state = active();
    expect(state).toMatchObject({ phase: 'active', streamId: 'match:m1', lastSeq: 10 });
    expect(state.match?.fighters).toHaveLength(6);
    expect(state.turn?.turnId).toBe('t1');
  });

  it('лобби-snapshot даёт фазу lobby и список игроков', () => {
    const state = applyServerMessage(initialGameState, fixtures.lobbySnapshot).state;
    expect(state.phase).toBe('lobby');
    expect(state.players).toHaveLength(2);
  });

  it('устаревшее состояние полностью заменяется новым snapshot, в том числе с меньшим seq', () => {
    let state = active();
    state = applyServerMessage(state, at(fixtures.shotResolved, 11)).state;
    expect(state.phase).toBe('resolving');

    state = applyServerMessage(state, { ...fixtures.activeSnapshot, seq: 3 }).state;
    expect(state).toMatchObject({ phase: 'active', lastSeq: 3, lastShot: undefined });
    expect(state.match?.fighters.every((f) => f.alive)).toBe(true);
  });
});

describe('последовательность seq', () => {
  it('применяет следующий по порядку seq', () => {
    const { state, ignored } = applyServerMessage(active(), at(fixtures.shotResolved, 11));
    expect(ignored).toBeUndefined();
    expect(state.lastSeq).toBe(11);
  });

  it('повтор и устаревшее событие не применяются', () => {
    const afterShot = applyServerMessage(active(), at(fixtures.shotResolved, 11)).state;
    const replay = applyServerMessage(afterShot, at(fixtures.shotResolved, 11));
    expect(replay.ignored).toBe('duplicate');
    expect(replay.state).toBe(afterShot);
    expect(applyServerMessage(afterShot, at(fixtures.turnOpened, 5)).ignored).toBe('duplicate');
  });

  it('пропуск seq не применяет событие, требует переподключения и замораживает поток до snapshot', () => {
    const gap = applyServerMessage(active(), at(fixtures.shotResolved, 13));
    expect(gap.reconnect).toBe(true);
    expect(gap.state.connection).toBe('reconnecting');
    expect(gap.state.phase).toBe('active');
    expect(gap.state.lastShot).toBeUndefined();

    const during = applyServerMessage(gap.state, at(fixtures.turnOpened, 14));
    expect(during).toMatchObject({ ignored: 'reconnecting', reconnect: false });

    const recovered = applyServerMessage(gap.state, { ...fixtures.activeSnapshot, seq: 20 }).state;
    expect(recovered).toMatchObject({ connection: 'open', lastSeq: 20 });
    expect(applyServerMessage(recovered, at(fixtures.shotResolved, 21)).ignored).toBeUndefined();
  });

  it('событие другого stream не меняет текущий матч', () => {
    const state = active();
    const foreign = applyServerMessage(state, { ...at(fixtures.matchFinished, 11), streamId: 'match:other' });
    expect(foreign.ignored).toBe('other_stream');
    expect(foreign.state).toBe(state);
  });

  it('события без snapshot игнорируются', () => {
    expect(applyServerMessage(initialGameState, fixtures.turnOpened).ignored).toBe('no_stream');
  });
});

describe('переходы фаз', () => {
  it('shot.resolved → resolving, убитый боец помечается мёртвым, turn.opened возвращает в active', () => {
    let state = applyServerMessage(active(), at(fixtures.shotResolved, 11)).state;
    expect(state.phase).toBe('resolving');
    expect(state.lastShot?.destroyedFighterId).toBe('b2');
    expect(state.match?.fighters.find((f) => f.id === 'b2')?.alive).toBe(false);
    expect(state.match?.fighters.filter((f) => f.alive)).toHaveLength(5);

    state = applyServerMessage(state, at(fixtures.turnOpened, 12)).state;
    expect(state).toMatchObject({ phase: 'active', turn: { turnId: 't2' } });
  });

  it('match.finished даёт итог и убирает ход', () => {
    const state = applyServerMessage(active(), at(fixtures.matchFinished, 11)).state;
    expect(state).toMatchObject({ phase: 'finished', turn: undefined, result: { winnerSide: 'A', reason: 'team_wiped' } });
  });

  it('после finished новые ходы и выстрелы не применяются', () => {
    const finished = applyServerMessage(active(), at(fixtures.matchFinished, 11)).state;
    expect(applyServerMessage(finished, at(fixtures.turnOpened, 12)).ignored).toBe('out_of_phase');
    expect(applyServerMessage(finished, at(fixtures.shotResolved, 12)).ignored).toBe('out_of_phase');
  });

  it('snapshot с фазой aborted даёт aborted без победителя', () => {
    const aborted: ServerMessage = { type: 'snapshot', streamId: 'match:m1', seq: 30, payload: { phase: 'aborted' } };
    expect(applyServerMessage(active(), aborted).state).toMatchObject({ phase: 'aborted', result: undefined, match: undefined });
  });

  it('room.changed обновляет игроков и сохраняет фазу матча', () => {
    const lobby = applyServerMessage(initialGameState, fixtures.lobbySnapshot).state;
    const next = applyServerMessage(lobby, {
      type: 'room.changed',
      streamId: 'room:r1',
      seq: 2,
      payload: { players: [{ guestId: 'g1', displayName: 'Аня', ready: false }] },
    }).state;
    expect(next.players).toHaveLength(1);
    expect(next.phase).toBe('lobby');
  });

  it('ошибка сохраняется как lastError и не двигает последовательность', () => {
    const state = applyServerMessage(active(), fixtures.error).state;
    expect(state.lastError).toEqual({ code: 'not_your_turn', message: 'сейчас ход соперника' });
    expect(state.lastSeq).toBe(10);
  });

  it('ошибка без streamId и seq принимается и до первого snapshot', () => {
    const outcome = applyRawMessage(initialGameState, { type: 'error', payload: { code: 'unauthorized', message: 'нет сессии' } });
    expect(outcome.rejected).toBeUndefined();
    expect(outcome.state.lastError?.code).toBe('unauthorized');
  });
});

describe('сброс и невалидный ввод', () => {
  it('смена комнаты очищает игровое состояние предыдущей, но не соединение', () => {
    const state = setConnection(active(), 'open');
    const left = leaveRoom(state);
    expect(left).toMatchObject({ phase: 'idle', connection: 'open', players: [] });
    expect(left.streamId).toBeUndefined();
    expect(left.match).toBeUndefined();
  });

  it('snapshot новой комнаты не сохраняет ничего от старой', () => {
    const shot = applyServerMessage(active(), at(fixtures.shotResolved, 11)).state;
    const next = applyServerMessage(shot, fixtures.lobbySnapshot).state;
    expect(next).toMatchObject({ phase: 'lobby', streamId: 'room:r1' });
    expect(next.match).toBeUndefined();
    expect(next.lastShot).toBeUndefined();
  });

  it('невалидное сообщение не меняет состояние и возвращает причину', () => {
    const state = active();
    const outcome = applyRawMessage(state, '{not json');
    expect(outcome.state).toBe(state);
    expect(outcome.rejected?.reason).toBe('invalid_json');
    expect(applyRawMessage(state, { ...fixtures.turnOpened, v: 99 }).rejected?.reason).toBe('unsupported_version');
  });
});

describe('toFieldState', () => {
  it('отдаёт геометрию только из серверного состояния', () => {
    expect(toFieldState(initialGameState)).toBeUndefined();
    const field = toFieldState(active(), 'a1');
    expect(field).toMatchObject({ width: 100, height: 60, selectedFighterId: 'a1' });
    expect(field?.fighters).toHaveLength(6);
  });
});
