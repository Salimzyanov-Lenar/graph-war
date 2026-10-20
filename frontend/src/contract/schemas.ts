import { z } from 'zod';
import type { Fighter, Obstacle, Side } from '../field/types';

/** Версия протокола, которую понимает клиент. Поле `v` в конверте необязательно, но если пришло — должно совпасть. */
export const PROTOCOL_VERSION = 1;

const side = z.enum(['A', 'B']);
const id = z.string().min(1).max(64);
const finiteNumber = z.number();

export const obstacleSchema = z.object({ x: finiteNumber, y: finiteNumber, r: z.number().positive() });
export const fighterSchema = z.object({ id, side, x: finiteNumber, y: finiteNumber, alive: z.boolean() });

// Проверка на этапе компиляции: схемы и типы поля не расходятся.
obstacleSchema satisfies z.ZodType<Obstacle>;
fighterSchema satisfies z.ZodType<Fighter>;
side satisfies z.ZodType<Side>;

export const mapSchema = z.object({
  width: z.number().positive(),
  height: z.number().positive(),
  obstacles: z.array(obstacleSchema).max(64),
  mapSeed: z.union([z.string(), z.number()]).optional(),
  generatorVersion: z.union([z.string(), z.number()]).optional(),
});

export const phaseSchema = z.enum(['lobby', 'active', 'resolving', 'finished', 'aborted']);

/** Известные значения из docs/contract.md. Схемы принимают любую непустую строку: незнакомая причина не должна отбрасывать целое сообщение (итог матча, выстрел). */
export const KNOWN_FINISH_REASONS = ['team_wiped', 'surrender', 'timeout_streak', 'forfeit_disconnect'] as const;
export const KNOWN_STOP_REASONS = ['out_of_field', 'discontinuity', 'non_finite', 'max_points'] as const;
export const KNOWN_ERROR_CODES = [
  'bad_expression',
  'domain_error_at_start',
  'not_your_turn',
  'unknown_fighter',
  'dead_fighter',
  'room_full',
  'room_expired',
  'unauthorized',
  'rate_limited',
  'duplicate_request',
  'stale_turn',
  'server_error',
] as const;

export const finishReasonSchema = z.string().min(1).max(64);
export const stopReasonSchema = z.string().min(1).max(64);
export const errorCodeSchema = z.string().min(1).max(64);

export const turnSchema = z.object({
  turnId: id,
  side: side.optional(),
  fighterId: id.optional(),
  deadlineAt: finiteNumber,
  serverTime: finiteNumber,
});

export const playerSchema = z.object({
  guestId: id,
  displayName: z.string().max(64),
  side: side.optional(),
  ready: z.boolean(),
});

export const shotSchema = z.object({
  shotId: id,
  turnId: id,
  points: z.array(z.tuple([finiteNumber, finiteNumber])).max(401),
  stopReason: stopReasonSchema,
  destroyedFighterId: id.optional(),
});

export const resultSchema = z.object({
  winnerSide: side.optional(),
  reason: finishReasonSchema,
});

export const matchBodySchema = z.object({
  matchId: id,
  yourSide: side,
  map: mapSchema,
  fighters: z.array(fighterSchema).max(6),
});

export const snapshotPayloadSchema = z.object({
  phase: phaseSchema,
  players: z.array(playerSchema).max(2).optional(),
  match: matchBodySchema.optional(),
  turn: turnSchema.optional(),
  lastShot: shotSchema.optional(),
  result: resultSchema.optional(),
});

const envelope = {
  v: z.number().int().optional(),
  streamId: id,
  seq: z.number().int().nonnegative(),
};

function message<T extends string, P extends z.ZodType>(type: T, payload: P) {
  return z.object({ ...envelope, type: z.literal(type), payload });
}

export const serverMessageSchema = z.discriminatedUnion('type', [
  message('snapshot', snapshotPayloadSchema),
  message('room.changed', z.object({ players: z.array(playerSchema).max(2) })),
  message('match.started', matchBodySchema),
  message('turn.opened', turnSchema),
  message('shot.resolved', shotSchema),
  message('match.finished', resultSchema),
  message('error', z.object({ code: errorCodeSchema, message: z.string().max(500) })),
]);

export type ErrorCode = z.infer<typeof errorCodeSchema>;
export type FinishReason = z.infer<typeof finishReasonSchema>;
export type StopReason = z.infer<typeof stopReasonSchema>;
export type Phase = z.infer<typeof phaseSchema>;
export type Turn = z.infer<typeof turnSchema>;
export type Player = z.infer<typeof playerSchema>;
export type Shot = z.infer<typeof shotSchema>;
export type MatchResult = z.infer<typeof resultSchema>;
export type MatchBody = z.infer<typeof matchBodySchema>;
export type SnapshotPayload = z.infer<typeof snapshotPayloadSchema>;
export type ServerMessage = z.infer<typeof serverMessageSchema>;

export type ClientCommand =
  | { type: 'room.ready'; payload: { ready: boolean } }
  | {
      type: 'match.fire';
      payload: { matchId: string; expectedTurnId: string; fighterId: string; expression: string; requestId: string };
    }
  | { type: 'match.surrender'; payload: { confirm: true } }
  | { type: 'stream.resync'; payload: { streamId: string } };
