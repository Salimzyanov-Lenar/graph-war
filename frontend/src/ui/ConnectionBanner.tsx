import type { GameState } from '../state/gameState';

/** Показывается только при проблемах: пока соединение в порядке, UI не шумит. */
export function ConnectionBanner({ state, onRetry }: { state: GameState; onRetry(): void }) {
  const over = state.phase === 'finished' || state.phase === 'aborted';
  if (state.connection === 'replaced') {
    return (
      <p className="conn-banner conn-lost" role="alert">
        Эта игра открыта в другом окне под тем же игроком — здесь соединение закрыто.
      </p>
    );
  }
  if (state.connection === 'reconnecting') {
    return (
      <p className="conn-banner" role="status">
        Соединение потеряно, переподключаемся… Выстрелы сейчас не отправляются.
      </p>
    );
  }
  if (state.connection === 'closed' && !over && state.phase !== 'idle') {
    return (
      <p className="conn-banner conn-lost" role="alert">
        Нет соединения с сервером.{' '}
        <button type="button" onClick={onRetry}>
          Повторить
        </button>
      </p>
    );
  }
  return null;
}
