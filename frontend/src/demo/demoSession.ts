import { fixtures } from '../contract/fixtures';
import type { ClientCommand } from '../contract/schemas';
import { createGameStore, type GameStore } from '../state/gameStore';
import { MockTransport } from '../transport/mockTransport';

export interface DemoSession {
  store: GameStore;
  send(command: ClientCommand): void;
  dispose(): void;
}

const STREAM = 'match:m1';
const RESOLVE_MS = 1200;

function arc(from: { x: number; y: number }, to: { x: number; y: number }): [number, number][] {
  const steps = 24;
  return Array.from({ length: steps + 1 }, (_, i) => {
    const t = i / steps;
    return [from.x + (to.x - from.x) * t, from.y + (to.y - from.y) * t + Math.sin(Math.PI * t) * 12] as [number, number];
  });
}

/** Сценарий вместо сервера для ручной проверки UI. Траектории и попадания здесь не считаются — только фиксированный ответ. */
export function createDemoSession(): DemoSession {
  let seq = fixtures.activeSnapshot.seq;
  let turnNumber = 1;
  const timers = new Set<ReturnType<typeof setTimeout>>();
  const later = (ms: number, fn: () => void) => {
    const timer = setTimeout(() => {
      timers.delete(timer);
      fn();
    }, ms);
    timers.add(timer);
  };

  const emit = (type: string, payload: unknown) => transport.emit({ type, streamId: STREAM, seq: ++seq, payload });
  const openTurn = (side: 'A' | 'B') => {
    const serverTime = Date.now();
    emit('turn.opened', { turnId: `t${++turnNumber}`, side, serverTime, deadlineAt: serverTime + 60_000 });
  };

  const transport: MockTransport = new MockTransport((command) => {
    if (command.type !== 'match.fire') return;
    const { expression, fighterId } = command.payload;
    const state = store.getState();
    if (command.payload.expectedTurnId !== state.turn?.turnId) {
      transport.emit({ type: 'error', payload: { code: 'stale_turn', message: '' } });
      return;
    }
    if (/\d\s*[a-z(]/i.test(expression)) {
      transport.emit({ type: 'error', payload: { code: 'bad_expression', message: '' } });
      return;
    }
    if (expression === 'ln(x)') {
      transport.emit({ type: 'error', payload: { code: 'domain_error_at_start', message: '' } });
      return;
    }
    const shooter = state.match?.fighters.find((f) => f.id === fighterId);
    const target = state.match?.fighters.find((f) => f.alive && f.side !== shooter?.side);
    if (!shooter) return;
    later(250, () => {
      // Заготовленный ответ сервера: дуга до первого живого врага (формула здесь не вычисляется).
      const points = target ? arc(shooter, target) : [[shooter.x, shooter.y], [100, shooter.y]];
      emit('shot.resolved', {
        shotId: `s${seq}`,
        turnId: state.turn?.turnId,
        points,
        stopReason: target ? 'hit' : 'out_of_field',
        destroyedFighterId: target?.id,
      });
      later(RESOLVE_MS, () => {
        const enemiesLeft = store.getState().match?.fighters.some((f) => f.alive && f.side !== shooter.side);
        if (!enemiesLeft) {
          emit('match.finished', { winnerSide: shooter.side, reason: 'team_wiped' });
          return;
        }
        const other = shooter.side === 'A' ? 'B' : 'A';
        openTurn(other);
        later(1500, () => openTurn(shooter.side));
      });
    });
  });

  const store = createGameStore(transport);
  store.connect();
  later(0, () => transport.emit(fixtures.activeSnapshot));

  return {
    store,
    send: (command) => transport.send(command),
    dispose() {
      timers.forEach(clearTimeout);
      timers.clear();
      store.dispose();
    },
  };
}
