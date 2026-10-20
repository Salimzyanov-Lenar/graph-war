import { describe, expect, it, vi } from 'vitest';
import { fixtures } from '../contract/fixtures';
import { MockTransport } from '../transport/mockTransport';
import { createGameStore } from './gameStore';

describe('createGameStore + MockTransport', () => {
  it('connect → open, сообщения проходят проверку схем и меняют состояние, подписчики уведомляются', async () => {
    const transport = new MockTransport();
    const store = createGameStore(transport);
    const listener = vi.fn();
    store.subscribe(listener);

    store.connect();
    expect(store.getState().connection).toBe('connecting');
    await Promise.resolve();
    expect(store.getState().connection).toBe('open');

    transport.emit(fixtures.activeSnapshot);
    expect(store.getState()).toMatchObject({ phase: 'active', streamId: 'match:m1' });
    expect(listener).toHaveBeenCalled();
  });

  it('мок отдаёт невалидные данные как реальный сокет: они отвергаются и копятся в rejected', () => {
    const transport = new MockTransport();
    const store = createGameStore(transport);
    transport.emit('{broken');
    transport.emit({ type: 'snapshot', streamId: 'x', seq: 1, payload: { phase: 'weird' } });
    expect(store.getState().phase).toBe('idle');
    expect(store.rejected.map((r) => r.reason)).toEqual(['invalid_json', 'invalid_message']);
  });

  it('при пропуске seq переподключается, а не шлёт команд; свежий snapshot восстанавливает состояние', async () => {
    const transport = new MockTransport();
    const connect = vi.spyOn(transport, 'connect');
    const close = vi.spyOn(transport, 'close');
    const store = createGameStore(transport);
    transport.emit(fixtures.activeSnapshot);
    transport.emit({ ...fixtures.shotResolved, seq: 15 });

    expect(close).toHaveBeenCalledTimes(1);
    expect(connect).toHaveBeenCalledTimes(1);
    expect(store.getState().connection).toBe('reconnecting');
    expect(transport.sent).toEqual([]);

    await Promise.resolve();
    expect(store.getState().connection).toBe('open');
    transport.emit({ ...fixtures.activeSnapshot, seq: 1 });
    expect(store.getState()).toMatchObject({ connection: 'open', lastSeq: 1, phase: 'active' });
  });

  it('обрыв соединения переводит в closed, dispose отписывается от транспорта', () => {
    const transport = new MockTransport();
    const store = createGameStore(transport);
    transport.open();
    transport.drop();
    expect(store.getState().connection).toBe('closed');

    store.dispose();
    transport.emit(fixtures.activeSnapshot);
    expect(store.getState().phase).toBe('idle');
  });

  it('leaveRoom сбрасывает игру, а событие старого потока после этого не применяется', () => {
    const transport = new MockTransport();
    const store = createGameStore(transport);
    transport.emit(fixtures.activeSnapshot);
    store.leaveRoom();
    expect(store.getState().phase).toBe('idle');
    transport.emit({ ...fixtures.shotResolved, seq: 11 });
    expect(store.getState().phase).toBe('idle');
  });

  describe('авто-переподключение', () => {
    function setup(maxAttempts = 3) {
      const transport = new MockTransport();
      const timers: { callback: () => void; ms: number; handle: number }[] = [];
      let next = 0;
      const connect = vi.spyOn(transport, 'connect');
      const store = createGameStore(transport, {
        reconnect: {
          policy: { baseMs: 100, maxMs: 1000, maxAttempts },
          random: () => 1,
          setTimer: (callback, ms) => {
            const handle = (next += 1);
            timers.push({ callback, ms, handle });
            return handle;
          },
          clearTimer: (handle) => {
            const index = timers.findIndex((t) => t.handle === handle);
            if (index >= 0) timers.splice(index, 1);
          },
        },
      });
      return { transport, store, timers, connect };
    }

    it('обрыв в матче → reconnecting, повтор по таймеру, snapshot восстанавливает состояние', () => {
      const { transport, store, timers, connect } = setup();
      store.connect();
      transport.open();
      transport.emit(fixtures.activeSnapshot);
      connect.mockClear();

      transport.drop();
      expect(store.getState().connection).toBe('reconnecting');
      expect(store.getState().phase).toBe('active');
      expect(timers.map((t) => t.ms)).toEqual([100]);

      timers[0]?.callback();
      expect(connect).toHaveBeenCalledTimes(1);
      transport.open();
      transport.emit({ ...fixtures.activeSnapshot, seq: 1 });
      expect(store.getState()).toMatchObject({ connection: 'open', phase: 'active', lastSeq: 1 });
    });

    it('после maxAttempts неудач сдаётся (closed); retry() даёт новую серию попыток', () => {
      const { transport, store, timers, connect } = setup(2);
      store.connect();
      transport.open();
      transport.drop();
      timers.shift()?.callback();
      transport.drop();
      timers.shift()?.callback();
      transport.drop();
      expect(store.getState().connection).toBe('closed');
      expect(timers).toHaveLength(0);

      connect.mockClear();
      store.retry();
      expect(store.getState().connection).toBe('reconnecting');
      expect(connect).toHaveBeenCalledTimes(1);
      transport.open();
      expect(store.getState().connection).toBe('open');
    });

    it('после завершения матча и после dispose не переподключается', () => {
      const first = setup();
      first.store.connect();
      first.transport.open();
      first.transport.emit({ ...fixtures.activeSnapshot, payload: { ...fixtures.activeSnapshot.payload, phase: 'aborted' } });
      first.transport.drop();
      expect(first.store.getState().connection).toBe('closed');
      expect(first.timers).toHaveLength(0);

      const second = setup();
      second.store.connect();
      second.transport.open();
      second.transport.drop();
      expect(second.timers).toHaveLength(1);
      second.store.dispose();
      expect(second.timers).toHaveLength(0);
    });

    it('переподключение из-за дыры в seq не запускает ещё и таймер обрыва', () => {
      const { transport, store, timers } = setup();
      transport.emit(fixtures.activeSnapshot);
      store.connect();
      transport.open();
      transport.emit({ ...fixtures.shotResolved, seq: 15 });
      expect(store.getState().connection).toBe('reconnecting');
      expect(timers).toHaveLength(0);
    });
  });
});
