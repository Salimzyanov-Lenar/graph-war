/** Справка по языку формул. Должна совпадать с docs/contract.md (раздел «Язык формул») — это проверяет тест. */
export const FORMULA_FUNCTIONS = ['sin', 'cos', 'sqrt', 'abs', 'ln', 'exp'] as const;
export const FORMULA_CONSTANTS = ['pi', 'e'] as const;
export const FORMULA_OPERATORS = ['+', '-', '*', '/', '^'] as const;
export const FORMULA_MAX_LENGTH = 100;

export const FORMULA_EXAMPLES: readonly { expression: string; note: string }[] = [
  { expression: 'x', note: 'прямая вверх-вперёд под 45°' },
  { expression: '0.5*x', note: 'пологая прямая' },
  { expression: '5*sin(x/3)', note: 'волна' },
  { expression: 'ln(x+1)', note: 'логарифм, который стартует' },
];

export const FORMULA_NOTES: readonly string[] = [
  'x — расстояние от стрелка в сторону соперника. Выстрел всегда начинается у бойца: постоянное слагаемое роли не играет (x^2 и x^2+5 — одно и то же).',
  'Стороны зеркальны: игрок справа стреляет справа налево, формула та же, x растёт в сторону соперника.',
  'Умножение пишется явно: 2*x, а не 2x. Дробная часть — через точку: 0.5.',
  'ln(x) не стартует (в точке x = 0 не определён), ln(x+1) — стартует.',
  'Углы в sin и cos — в радианах.',
  'Формула, которая разобралась без ошибок, не обязательно долетит до цели.',
];
