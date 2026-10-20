import { describe, expect, it } from 'vitest';
import { fixtures } from './fixtures';
import { parseServerMessage } from './parse';

describe('parseServerMessage', () => {
  it('принимает все контрактные фикстуры (в виде объекта и в виде JSON-строки)', () => {
    for (const [name, fixture] of Object.entries(fixtures)) {
      expect(parseServerMessage(fixture), name).toMatchObject({ ok: true });
      expect(parseServerMessage(JSON.stringify(fixture)), name).toMatchObject({ ok: true });
    }
  });

  it('отвергает невалидный JSON', () => {
    expect(parseServerMessage('{oops')).toMatchObject({ ok: false, reason: 'invalid_json' });
  });

  it('отвергает неподдерживаемую версию протокола и принимает поддерживаемую', () => {
    expect(parseServerMessage({ ...fixtures.turnOpened, v: 2 })).toMatchObject({ ok: false, reason: 'unsupported_version' });
    expect(parseServerMessage({ ...fixtures.turnOpened, v: 1 })).toMatchObject({ ok: true });
  });

  it('отвергает неизвестный тип, отсутствие streamId/seq и неверные поля payload', () => {
    expect(parseServerMessage({ ...fixtures.turnOpened, type: 'match.exploded' })).toMatchObject({ ok: false, reason: 'invalid_message' });
    expect(parseServerMessage({ type: 'turn.opened', payload: fixtures.turnOpened.payload })).toMatchObject({ ok: false });
    expect(parseServerMessage({ ...fixtures.turnOpened, seq: -1 })).toMatchObject({ ok: false });
    expect(parseServerMessage({ ...fixtures.turnOpened, seq: 1.5 })).toMatchObject({ ok: false });
    expect(parseServerMessage({ ...fixtures.turnOpened, payload: { turnId: 't', deadlineAt: 'скоро', serverTime: 1 } })).toMatchObject({ ok: false });
  });

  it('отвергает нечисловые координаты, NaN/Infinity и лишние бойцы', () => {
    const bad = (payload: unknown) => parseServerMessage({ ...fixtures.shotResolved, payload });
    const base = fixtures.shotResolved.payload;
    expect(bad({ ...base, points: [[1, 'x']] })).toMatchObject({ ok: false });
    expect(bad({ ...base, points: [[1, Infinity]] })).toMatchObject({ ok: false });
    expect(bad({ ...base, points: [[Number.NaN, 1]] })).toMatchObject({ ok: false });
    expect(bad({ ...base, points: Array.from({ length: 402 }, () => [0, 0]) })).toMatchObject({ ok: false });

    const snapshot = structuredClone(fixtures.activeSnapshot);
    snapshot.payload.match.fighters.push({ id: 'x7', side: 'A', x: 1, y: 1, alive: true });
    expect(parseServerMessage(snapshot)).toMatchObject({ ok: false });
  });

  it('не падает на мусоре: null, число, массив', () => {
    for (const junk of [null, 42, [], 'null', undefined]) {
      expect(parseServerMessage(junk)).toMatchObject({ ok: false });
    }
  });

  it('не отбрасывает сообщение из-за незнакомой причины остановки или кода ошибки', () => {
    expect(parseServerMessage({ ...fixtures.shotResolved, payload: { ...fixtures.shotResolved.payload, stopReason: 'obstacle' } })).toMatchObject({ ok: true });
    expect(parseServerMessage({ ...fixtures.error, payload: { code: 'brand_new_code', message: 'x' } })).toMatchObject({ ok: true });
  });
});
