import { useId, useState, type FormEvent } from 'react';
import type { ClientCommand } from '../contract/schemas';
import { FORMULA_EXAMPLES, FORMULA_FUNCTIONS, FORMULA_CONSTANTS, FORMULA_MAX_LENGTH, FORMULA_NOTES, FORMULA_OPERATORS } from '../input/formulaHelp';
import {
  FIRE_BLOCK_TEXT,
  buildFireCommand,
  errorText,
  fireBlock,
  myFighters,
  pendingFor,
  selectableFighter,
  type PendingFire,
} from '../input/fireInput';
import type { GameState } from '../state/gameState';

interface FirePanelProps {
  state: GameState;
  selectedId: string | undefined;
  onSelect(fighterId: string): void;
  send(command: ClientCommand): void;
  newRequestId?: () => string;
}

export function FirePanel({ state, selectedId, onSelect, send, newRequestId = () => crypto.randomUUID() }: FirePanelProps) {
  const [expression, setExpression] = useState('');
  const [pending, setPending] = useState<PendingFire>();
  const inputId = useId();
  const selected = selectableFighter(state, selectedId)?.id;
  const block = fireBlock(state, selected, expression, pending);
  const showError = state.lastError && !(block === 'pending');

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (fireBlock(state, selected, expression, pending)) return;
    const requestId = newRequestId();
    const command = buildFireCommand(state, selected, expression, requestId);
    if (!command) return;
    setPending(pendingFor(state, requestId));
    send(command);
  };

  return (
    <form className="fire-panel" onSubmit={submit} aria-label="Выстрел">
      <fieldset>
        <legend>Ваши бойцы</legend>
        <ul className="fighter-list">
          {myFighters(state).map((fighter) => (
            <li key={fighter.id}>
              <button
                type="button"
                aria-pressed={fighter.id === selected}
                disabled={!fighter.alive}
                onClick={() => onSelect(fighter.id)}
              >
                {fighter.id.toUpperCase()} — {fighter.alive ? 'жив' : 'уничтожен'}
              </button>
            </li>
          ))}
        </ul>
      </fieldset>

      <label htmlFor={inputId}>Формула y = f(x)</label>
      <input
        id={inputId}
        value={expression}
        maxLength={FORMULA_MAX_LENGTH}
        autoComplete="off"
        spellCheck={false}
        placeholder="например: 5*sin(x/3)"
        onChange={(event) => setExpression(event.target.value)}
      />
      <button type="submit" disabled={block !== undefined}>
        Выстрелить
      </button>

      <p className="fire-status" role="status">
        {block ? FIRE_BLOCK_TEXT[block] : 'Готово к выстрелу.'}
      </p>
      {showError && state.lastError && <p role="alert">{errorText(state.lastError)}</p>}

      <details className="formula-help">
        <summary>Справка по формулам</summary>
        <p>
          Операторы: <code>{FORMULA_OPERATORS.join(' ')}</code>. Функции: <code>{FORMULA_FUNCTIONS.join(' ')}</code>.
          Константы: <code>{FORMULA_CONSTANTS.join(' ')}</code>. До {FORMULA_MAX_LENGTH} символов.
        </p>
        <ul>
          {FORMULA_NOTES.map((note) => (
            <li key={note}>{note}</li>
          ))}
        </ul>
        <p>Примеры:</p>
        <ul>
          {FORMULA_EXAMPLES.map(({ expression: example, note }) => (
            <li key={example}>
              <button type="button" className="example" onClick={() => setExpression(example)}>
                <code>{example}</code>
              </button>{' '}
              — {note}
            </li>
          ))}
        </ul>
      </details>
    </form>
  );
}
