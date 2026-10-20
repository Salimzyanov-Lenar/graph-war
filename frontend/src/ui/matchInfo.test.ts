import { describe, expect, it } from 'vitest';
import { fixtures } from '../contract/fixtures';
import { applyServerMessage, initialGameState, type GameState } from '../state/gameState';
import { aliveCounts, formatClock, matchOutcome, remainingMs } from './matchInfo';

const active = (): GameState => applyServerMessage(initialGameState, fixtures.activeSnapshot).state;
const finished = (state: GameState, winnerSide: 'A' | 'B' | undefined, reason: string): GameState => ({
  ...state,
  phase: 'finished',
  result: { winnerSide, reason },
});

describe('remainingMs', () => {
  const turn = { serverTime: 1_000_000, deadlineAt: 1_060_000 };

  it('считает от серверного времени и локального монотонного хода, а не от часов компьютера', () => {
    expect(remainingMs(turn, 500, 500)).toBe(60_000);
    expect(remainingMs(turn, 500, 20_500)).toBe(40_000);
  });

  it('не уходит в минус: локальный ноль сам по себе ничего не завершает', () => {
    expect(remainingMs(turn, 0, 999_999)).toBe(0);
  });
});

describe('formatClock', () => {
  it('округляет вверх до секунды', () => {
    expect(formatClock(60_000)).toBe('1:00');
    expect(formatClock(59_001)).toBe('1:00');
    expect(formatClock(9_100)).toBe('0:10');
    expect(formatClock(0)).toBe('0:00');
  });
});

describe('aliveCounts', () => {
  it('считает живых и всех бойцов каждой стороны', () => {
    expect(aliveCounts(active())).toEqual({ A: { alive: 3, total: 3 }, B: { alive: 3, total: 3 } });
    expect(aliveCounts(initialGameState)).toEqual({ A: { alive: 0, total: 0 }, B: { alive: 0, total: 0 } });
  });
});

describe('matchOutcome', () => {
  const mine = active().match!.yourSide;
  const other = mine === 'A' ? 'B' : 'A';

  it('пока матч идёт, исхода нет', () => {
    expect(matchOutcome(active())).toBeUndefined();
  });

  it('победа и поражение различаются, причины сдачи, таймаутов и отключения — тоже', () => {
    expect(matchOutcome(finished(active(), mine, 'team_wiped'))).toMatchObject({ kind: 'win', title: 'Победа' });
    expect(matchOutcome(finished(active(), other, 'team_wiped'))).toMatchObject({ kind: 'loss', title: 'Поражение' });
    expect(matchOutcome(finished(active(), other, 'surrender'))?.detail).toBe('Вы сдались.');
    expect(matchOutcome(finished(active(), mine, 'surrender'))?.detail).toBe('Соперник сдался.');
    expect(matchOutcome(finished(active(), other, 'timeout_streak'))?.detail).toContain('трижды');
    expect(matchOutcome(finished(active(), mine, 'forfeit_disconnect'))?.detail).toBe('Соперник отключился.');
  });

  it('aborted и итог без победителя — отдельные исходы, не ничья по числу промахов', () => {
    expect(matchOutcome({ ...active(), phase: 'aborted' })).toMatchObject({ kind: 'aborted' });
    expect(matchOutcome(finished(active(), undefined, 'x'))).toMatchObject({ kind: 'no_winner' });
  });

  it('незнакомая причина показывается как есть', () => {
    expect(matchOutcome(finished(active(), mine, 'new_reason'))?.detail).toContain('new_reason');
  });
});
