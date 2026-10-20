import { applyRawMessage, initialGameState, leaveRoom, setConnection, type GameState, type Outcome } from './gameState';
import type { Transport } from '../transport/transport';

export interface GameStore {
  getState(): GameState;
  subscribe(listener: () => void): () => void;
  connect(): void;
  leaveRoom(): void;
  dispose(): void;
  /** Сообщения, отвергнутые проверкой схемы (для диагностики и UI-индикатора). */
  readonly rejected: readonly NonNullable<Outcome['rejected']>[];
}

/** Совместим с React `useSyncExternalStore`. Покадровая анимация через это состояние не идёт. */
export function createGameStore(transport: Transport): GameStore {
  let state = initialGameState;
  const listeners = new Set<() => void>();
  const rejected: NonNullable<Outcome['rejected']>[] = [];

  const set = (next: GameState) => {
    if (next === state) return;
    state = next;
    listeners.forEach((l) => l());
  };

  const unsubscribe = transport.subscribe({
    onOpen: () => set(setConnection(state, 'open')),
    onClose: () => set(setConnection(state, 'closed')),
    onMessage: (raw) => {
      const outcome = applyRawMessage(state, raw);
      if (outcome.rejected) rejected.push(outcome.rejected);
      set(outcome.state);
      if (outcome.resync && state.streamId) {
        transport.send({ type: 'stream.resync', payload: { streamId: state.streamId } });
      }
    },
  });

  return {
    getState: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    connect() {
      set(setConnection(state, 'connecting'));
      transport.connect();
    },
    leaveRoom: () => set(leaveRoom(state)),
    dispose: unsubscribe,
    rejected,
  };
}
