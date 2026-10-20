import { PROTOCOL_VERSION, serverMessageSchema, type ServerMessage } from './schemas';

export type ParseResult =
  | { ok: true; message: ServerMessage }
  | { ok: false; reason: 'invalid_json' | 'unsupported_version' | 'invalid_message'; detail: string };

/** Единственная точка входа для сетевых данных: сырой текст или уже разобранное значение (мок) → проверенное сообщение. */
export function parseServerMessage(raw: unknown): ParseResult {
  let value: unknown = raw;
  if (typeof raw === 'string') {
    try {
      value = JSON.parse(raw);
    } catch {
      return { ok: false, reason: 'invalid_json', detail: 'не удалось разобрать JSON' };
    }
  }

  if (typeof value === 'object' && value !== null && 'v' in value) {
    const version = (value as { v: unknown }).v;
    if (version !== undefined && version !== PROTOCOL_VERSION) {
      return { ok: false, reason: 'unsupported_version', detail: `версия протокола ${String(version)}` };
    }
  }

  const parsed = serverMessageSchema.safeParse(value);
  if (!parsed.success) {
    return { ok: false, reason: 'invalid_message', detail: parsed.error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join('; ') };
  }
  return { ok: true, message: parsed.data };
}
