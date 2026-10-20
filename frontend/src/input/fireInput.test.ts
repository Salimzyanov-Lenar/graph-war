import { describe, expect, it } from 'vitest';
import { fixtures } from '../contract/fixtures';
import { applyServerMessage, initialGameState, setConnection } from '../state/gameState';
import { fireBlock } from './fireInput';

describe('fireBlock и соединение', () => {
  const active = applyServerMessage(initialGameState, fixtures.activeSnapshot).state;
  const fighter = active.match?.fighters.find((f) => f.side === active.match?.yourSide && f.alive)?.id;

  it('при обрыве выстрел блокируется как offline, а не откладывается', () => {
    for (const connection of ['reconnecting', 'closed'] as const) {
      expect(fireBlock(setConnection(active, connection), fighter, 'x', undefined)).toBe('offline');
    }
  });

  it('при открытом соединении офлайн-блокировки нет', () => {
    expect(fireBlock(setConnection(active, 'open'), fighter, 'x', undefined)).not.toBe('offline');
  });
});
