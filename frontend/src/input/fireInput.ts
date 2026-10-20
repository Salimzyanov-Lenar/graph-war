import type { ClientCommand } from '../contract/schemas';
import type { Fighter } from '../field/types';
import type { GameState } from '../state/gameState';

export type FireBlock = 'not_active' | 'resolving' | 'finished' | 'not_your_turn' | 'no_fighter' | 'empty' | 'pending' | 'offline';

export function myFighters(state: GameState): Fighter[] {
  const side = state.match?.yourSide;
  return side ? (state.match?.fighters.filter((f) => f.side === side) ?? []) : [];
}

/** Выбрать можно только своего живого бойца. */
export function selectableFighter(state: GameState, fighterId: string | undefined): Fighter | undefined {
  if (!fighterId) return undefined;
  return myFighters(state).find((f) => f.id === fighterId && f.alive);
}

/** Чей ход. Если сервер не прислал `side`, пробуем вывести его из `fighterId`; иначе считаем, что можно, — решает сервер (`not_your_turn`). */
export function isMyTurn(state: GameState): boolean {
  const turn = state.turn;
  const mine = state.match?.yourSide;
  if (!turn || !mine) return false;
  if (turn.side) return turn.side === mine;
  const fighter = turn.fighterId ? state.match?.fighters.find((f) => f.id === turn.fighterId) : undefined;
  return fighter ? fighter.side === mine : true;
}

export interface PendingFire {
  requestId: string;
  turnId: string;
  shotId?: string;
  error: GameState['lastError'];
}

export function pendingFor(state: GameState, requestId: string): PendingFire | undefined {
  if (!state.turn) return undefined;
  return { requestId, turnId: state.turn.turnId, shotId: state.lastShot?.shotId, error: state.lastError };
}

/** Отправка заблокирована, пока сервер не подтвердил её: новый выстрел, смена хода или ответ-ошибка. */
export function isPendingResolved(state: GameState, pending: PendingFire): boolean {
  return (
    state.turn?.turnId !== pending.turnId || state.lastShot?.shotId !== pending.shotId || state.lastError !== pending.error
  );
}

export function fireBlock(
  state: GameState,
  selectedId: string | undefined,
  expression: string,
  pending: PendingFire | undefined,
): FireBlock | undefined {
  if (state.phase === 'finished' || state.phase === 'aborted') return 'finished';
  if (state.phase === 'resolving') return 'resolving';
  if (state.phase !== 'active' || !state.match) return 'not_active';
  if (state.connection === 'reconnecting' || state.connection === 'closed') return 'offline';
  if (!isMyTurn(state)) return 'not_your_turn';
  if (pending && !isPendingResolved(state, pending)) return 'pending';
  if (!selectableFighter(state, selectedId)) return 'no_fighter';
  if (expression.trim() === '') return 'empty';
  return undefined;
}

export const FIRE_BLOCK_TEXT: Record<FireBlock, string> = {
  not_active: 'Матч ещё не начался.',
  resolving: 'Идёт воспроизведение выстрела.',
  finished: 'Матч завершён.',
  not_your_turn: 'Сейчас ход соперника.',
  no_fighter: 'Выберите своего живого бойца.',
  empty: 'Введите формулу.',
  pending: 'Выстрел отправлен, ждём ответ сервера…',
  offline: 'Нет соединения с сервером — выстрел не отправлен.',
};

/** Клиент присылает только fighterId и expression; координат, попаданий и победителя в команде нет. */
export function buildFireCommand(
  state: GameState,
  selectedId: string | undefined,
  expression: string,
  requestId: string,
): ClientCommand | undefined {
  const fighter = selectableFighter(state, selectedId);
  const turn = state.turn;
  if (!fighter || !turn || !state.match) return undefined;
  return {
    type: 'match.fire',
    payload: {
      matchId: state.match.matchId,
      expectedTurnId: turn.turnId,
      fighterId: fighter.id,
      expression: expression.trim(),
      requestId,
    },
  };
}

const ERROR_TEXT: Record<string, string> = {
  bad_expression: 'Формула не распознана. Проверьте скобки, знаки * и названия функций.',
  domain_error_at_start: 'Формула не определена в точке стрелка (x = 0). Например, вместо ln(x) используйте ln(x+1). Ход не потрачен.',
  not_your_turn: 'Сейчас не ваш ход.',
  stale_turn: 'Ход уже сменился. Повторите выстрел в свой ход.',
  unknown_fighter: 'Такого бойца нет.',
  dead_fighter: 'Этот боец уничтожен — выберите другого.',
  rate_limited: 'Слишком часто. Подождите немного.',
  duplicate_request: 'Этот выстрел уже отправлен.',
  unauthorized: 'Сессия недействительна. Обновите страницу.',
  server_error: 'Ошибка сервера. Попробуйте ещё раз.',
};

export function errorText(error: { code: string; message: string }): string {
  return ERROR_TEXT[error.code] ?? (error.message || `Ошибка: ${error.code}`);
}
