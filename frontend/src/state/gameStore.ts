import { applyRawMessage, initialGameState, leaveRoom, setConnection, type GameState, type Outcome } from './gameState';
import { backoffDelayMs, DEFAULT_RECONNECT, type ReconnectPolicy } from '../transport/reconnect';
import type { Transport } from '../transport/transport';

/** Коды закрытия /ws (backend/internal/rooms/clock.go): повторять подключение бессмысленно. */
const CLOSE_REPLACED = 4001;
const CLOSE_NO_ROOM = 4003;

export interface ReconnectOptions {
  policy?: ReconnectPolicy;
  random?: () => number;
  setTimer?: (callback: () => void, ms: number) => unknown;
  clearTimer?: (handle: unknown) => void;
}

export interface StoreOptions {
  /** Авто-переподключение при неожиданном обрыве. Без опции обрыв просто переводит соединение в closed. */
  reconnect?: ReconnectOptions;
}

export interface GameStore {
  getState(): GameState;
  subscribe(listener: () => void): () => void;
  connect(): void;
  leaveRoom(): void;
  /** Ручная попытка после исчерпания авто-переподключений. */
  retry(): void;
  dispose(): void;
  /** Сообщения, отвергнутые проверкой схемы (для диагностики и UI-индикатора). */
  readonly rejected: readonly NonNullable<Outcome['rejected']>[];
}

/** Совместим с React `useSyncExternalStore`. Покадровая анимация через это состояние не идёт. */
export function createGameStore(transport: Transport, options: StoreOptions = {}): GameStore {
  let state = initialGameState;
  const listeners = new Set<() => void>();
  const rejected: NonNullable<Outcome['rejected']>[] = [];

  const set = (next: GameState) => {
    if (next === state) return;
    state = next;
    listeners.forEach((l) => l());
  };

  const auto = options.reconnect;
  const policy = auto?.policy ?? DEFAULT_RECONNECT;
  const setTimer = auto?.setTimer ?? ((callback, ms) => setTimeout(callback, ms));
  const clearTimer = auto?.clearTimer ?? ((handle) => clearTimeout(handle as ReturnType<typeof setTimeout>));
  let attempts = 0;
  let timer: unknown;
  let intentional = false;
  let disposed = false;

  const cancelTimer = () => {
    if (timer !== undefined) clearTimer(timer);
    timer = undefined;
  };

  const closeTransport = () => {
    intentional = true;
    try {
      transport.close();
    } finally {
      intentional = false;
    }
  };

  const finished = () => state.phase === 'finished' || state.phase === 'aborted';

  const scheduleReconnect = () => {
    if (attempts >= policy.maxAttempts) {
      set(setConnection(state, 'closed'));
      return;
    }
    attempts += 1;
    set(setConnection(state, 'reconnecting'));
    timer = setTimer(() => {
      timer = undefined;
      if (!disposed) transport.connect();
    }, backoffDelayMs(attempts, policy, auto?.random));
  };

  const unsubscribe = transport.subscribe({
    onOpen: () => {
      attempts = 0;
      cancelTimer();
      set(setConnection(state, 'open'));
    },
    onClose: (code) => {
      if (intentional || disposed) return;
      if (code === CLOSE_REPLACED) {
        cancelTimer();
        set(setConnection(state, 'replaced'));
        return;
      }
      if (code === CLOSE_NO_ROOM) {
        cancelTimer();
        set(setConnection(state, 'closed'));
        return;
      }
      if (auto && !finished() && state.connection !== 'idle') {
        scheduleReconnect();
        return;
      }
      set(setConnection(state, 'closed'));
    },
    onMessage: (raw) => {
      const outcome = applyRawMessage(state, raw);
      if (outcome.rejected) rejected.push(outcome.rejected);
      set(outcome.state);
      if (outcome.reconnect) {
        // stream.resync в контракте нет: при дыре в seq переподключаемся, /ws первым сообщением шлёт snapshot.
        closeTransport();
        set(setConnection(state, 'reconnecting'));
        transport.connect();
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
    retry() {
      if (disposed || state.connection !== 'closed') return;
      attempts = 0;
      cancelTimer();
      set(setConnection(state, 'reconnecting'));
      transport.connect();
    },
    dispose() {
      disposed = true;
      cancelTimer();
      unsubscribe();
    },
    rejected,
  };
}
