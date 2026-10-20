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

  it('при пропуске seq отправляет stream.resync с текущим streamId', () => {
    const transport = new MockTransport();
    const store = createGameStore(transport);
    transport.emit(fixtures.activeSnapshot);
    transport.emit({ ...fixtures.shotResolved, seq: 15 });

    expect(store.getState().connection).toBe('resyncing');
    expect(transport.sent).toEqual([{ type: 'stream.resync', payload: { streamId: 'match:m1' } }]);
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
});
