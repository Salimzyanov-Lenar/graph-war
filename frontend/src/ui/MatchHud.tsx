import { useEffect, useMemo, useState } from 'react';
import type { ClientCommand } from '../contract/schemas';
import { isMyTurn } from '../input/fireInput';
import type { GameState } from '../state/gameState';
import { aliveCounts, formatClock, matchOutcome, remainingMs } from './matchInfo';

interface MatchHudProps {
  state: GameState;
  send(command: ClientCommand): void;
}

const TICK_MS = 250;

/** Остаток хода: обновляется раз в четверть секунды как обычный текст, а не покадрово через сцену. */
function useTurnRemaining(state: GameState): number | undefined {
  const turn = state.phase === 'active' ? state.turn : undefined;
  const receivedAt = useMemo(() => performance.now(), [turn]);
  const [now, setNow] = useState(() => performance.now());

  useEffect(() => {
    if (!turn) return;
    setNow(performance.now());
    const timer = setInterval(() => setNow(performance.now()), TICK_MS);
    return () => clearInterval(timer);
  }, [turn]);

  return turn ? remainingMs(turn, receivedAt, now) : undefined;
}

export function MatchHud({ state, send }: MatchHudProps) {
  const remaining = useTurnRemaining(state);
  const [confirming, setConfirming] = useState(false);
  const [surrendered, setSurrendered] = useState(false);
  const counts = aliveCounts(state);
  const mine = state.match?.yourSide;
  const outcome = matchOutcome(state);
  const running = state.phase === 'active' || state.phase === 'resolving';

  if (!state.match || !mine) return null;
  const rival = mine === 'A' ? 'B' : 'A';

  const surrender = () => {
    setConfirming(false);
    setSurrendered(true);
    send({ type: 'match.surrender', payload: { confirm: true } });
  };

  let turnText = '';
  if (state.phase === 'active') turnText = isMyTurn(state) ? 'Ваш ход' : 'Ход соперника';
  else if (state.phase === 'resolving') turnText = 'Выстрел…';

  return (
    <section className="hud" aria-label="Состояние матча">
      <p className="hud-teams">
        <span>
          Вы ({mine}): <strong>{counts[mine].alive}</strong>/{counts[mine].total}
        </span>
        <span>
          Соперник ({rival}): <strong>{counts[rival].alive}</strong>/{counts[rival].total}
        </span>
      </p>

      {turnText && (
        <p className="hud-turn" role="status">
          {turnText}
          {remaining !== undefined && (
            <>
              {' · '}
              <span className="hud-clock" title="Время на ход по часам сервера">
                {formatClock(remaining)}
              </span>
              {remaining === 0 && ' (ждём решения сервера)'}
            </>
          )}
        </p>
      )}

      {outcome && (
        <div className={`hud-outcome hud-${outcome.kind}`} role="status">
          <strong>{outcome.title}</strong>
          <span>{outcome.detail}</span>
        </div>
      )}

      {running && !confirming && (
        <button type="button" className="hud-surrender" disabled={surrendered} onClick={() => setConfirming(true)}>
          {surrendered ? 'Сдача отправлена…' : 'Сдаться'}
        </button>
      )}
      {running && confirming && (
        <div className="hud-confirm" role="group" aria-label="Подтверждение сдачи">
          <span>Сдаться и проиграть матч?</span>
          <button type="button" onClick={surrender}>
            Да, сдаться
          </button>
          <button type="button" onClick={() => setConfirming(false)}>
            Отмена
          </button>
        </div>
      )}
    </section>
  );
}
