import { describe, expect, it } from 'vitest';
import { backoffDelayMs, DEFAULT_RECONNECT } from './reconnect';

describe('backoffDelayMs', () => {
  it('растёт экспоненциально и ограничен maxMs', () => {
    const top = () => 1;
    expect(backoffDelayMs(1, DEFAULT_RECONNECT, top)).toBe(500);
    expect(backoffDelayMs(2, DEFAULT_RECONNECT, top)).toBe(1000);
    expect(backoffDelayMs(3, DEFAULT_RECONNECT, top)).toBe(2000);
    expect(backoffDelayMs(20, DEFAULT_RECONNECT, top)).toBe(DEFAULT_RECONNECT.maxMs);
  });

  it('jitter даёт значение от половины до целого', () => {
    expect(backoffDelayMs(3, DEFAULT_RECONNECT, () => 0)).toBe(1000);
    expect(backoffDelayMs(3, DEFAULT_RECONNECT, () => 1)).toBe(2000);
  });
});
