import type { Turn } from '../contract/schemas';
import type { GameState } from '../state/gameState';

export interface SideCount {
  alive: number;
  total: number;
}

export function aliveCounts(state: GameState): Record<'A' | 'B', SideCount> {
  const counts = { A: { alive: 0, total: 0 }, B: { alive: 0, total: 0 } };
  for (const fighter of state.match?.fighters ?? []) {
    counts[fighter.side].total += 1;
    if (fighter.alive) counts[fighter.side].alive += 1;
  }
  return counts;
}

/**
 * Сколько осталось до дедлайна по оценке серверных часов: сервер сообщил `serverTime` в момент `receivedAt`
 * (монотонные часы браузера), дальше идёт локальный интервал. Часы компьютера и фоновая вкладка времени не добавляют.
 */
export function remainingMs(turn: Pick<Turn, 'deadlineAt' | 'serverTime'>, receivedAt: number, now: number): number {
  const serverNow = turn.serverTime + (now - receivedAt);
  return Math.max(0, turn.deadlineAt - serverNow);
}

export function formatClock(ms: number): string {
  const seconds = Math.ceil(ms / 1000);
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
}

export type Outcome = 'win' | 'loss' | 'aborted' | 'no_winner';

export interface MatchOutcome {
  kind: Outcome;
  title: string;
  detail: string;
}

const REASONS: Record<string, { win: string; loss: string }> = {
  team_wiped: { win: 'Все бойцы соперника уничтожены.', loss: 'Все ваши бойцы уничтожены.' },
  surrender: { win: 'Соперник сдался.', loss: 'Вы сдались.' },
  timeout_streak: { win: 'Соперник трижды подряд пропустил ход.', loss: 'Вы трижды подряд пропустили ход.' },
  forfeit_disconnect: { win: 'Соперник отключился.', loss: 'Вы слишком долго были не в сети.' },
};

/** Разные исходы — разные заголовки и пояснения; незнакомая причина показывается как есть. */
export function matchOutcome(state: GameState): MatchOutcome | undefined {
  if (state.phase === 'aborted') {
    return { kind: 'aborted', title: 'Матч прерван', detail: 'Победителя нет: матч остановлен сервером.' };
  }
  if (state.phase !== 'finished' || !state.result) return undefined;
  const { winnerSide, reason } = state.result;
  const mine = state.match?.yourSide;
  if (!winnerSide || !mine) {
    return { kind: 'no_winner', title: 'Матч окончен', detail: `Без победителя (${reason}).` };
  }
  const won = winnerSide === mine;
  const text = REASONS[reason];
  return {
    kind: won ? 'win' : 'loss',
    title: won ? 'Победа' : 'Поражение',
    detail: text ? text[won ? 'win' : 'loss'] : `Причина: ${reason}.`,
  };
}
