import { sampleField } from '../field/fixtures';
import type { ServerMessage } from './schemas';

const MATCH = 'match:m1';

const map = { width: sampleField.width, height: sampleField.height, obstacles: [...sampleField.obstacles] };
const freshFighters = sampleField.fighters.map((f) => ({ ...f, alive: true }));

/** Контрактные фикстуры (docs/contract.md v0.1). Мок-транспорт отдаёт их через те же схемы, что и реальные сообщения. */
export const fixtures = {
  lobbySnapshot: {
    type: 'snapshot',
    streamId: 'room:r1',
    seq: 1,
    payload: {
      phase: 'lobby',
      players: [
        { guestId: 'g1', displayName: 'Аня', side: 'A', ready: true },
        { guestId: 'g2', displayName: 'Борис', side: 'B', ready: false },
      ],
    },
  },
  activeSnapshot: {
    type: 'snapshot',
    streamId: MATCH,
    seq: 10,
    payload: {
      phase: 'active',
      match: { matchId: 'm1', yourSide: 'A', map, fighters: freshFighters },
      turn: { turnId: 't1', side: 'A', deadlineAt: 1_760_000_060_000, serverTime: 1_760_000_000_000 },
    },
  },
  turnOpened: {
    type: 'turn.opened',
    streamId: MATCH,
    seq: 11,
    payload: { turnId: 't2', side: 'B', deadlineAt: 1_760_000_120_000, serverTime: 1_760_000_060_500 },
  },
  shotResolved: {
    type: 'shot.resolved',
    streamId: MATCH,
    seq: 11,
    payload: {
      shotId: 's1',
      turnId: 't1',
      points: [[12, 30], [20, 31], [40, 33], [88, 30]],
      stopReason: 'hit',
      destroyedFighterId: 'b2',
    },
  },
  matchFinished: {
    type: 'match.finished',
    streamId: MATCH,
    seq: 12,
    payload: { winnerSide: 'A', reason: 'team_wiped' },
  },
  error: {
    type: 'error',
    streamId: MATCH,
    seq: 13,
    payload: { code: 'not_your_turn', message: 'сейчас ход соперника' },
  },
} satisfies Record<string, ServerMessage>;

export type FixtureName = keyof typeof fixtures;
