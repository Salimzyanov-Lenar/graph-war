import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { FORMULA_CONSTANTS, FORMULA_FUNCTIONS, FORMULA_MAX_LENGTH } from './formulaHelp';

const contract = readFileSync(resolve(__dirname, '../../../docs/contract.md'), 'utf-8');

describe('справка по формулам совпадает с docs/contract.md', () => {
  it('список функций', () => {
    expect(contract).toContain(`функции \`${FORMULA_FUNCTIONS.join(' ')}\``);
  });

  it('константы и лимит длины', () => {
    for (const name of FORMULA_CONSTANTS) expect(contract).toContain(`\`${name}\``);
    expect(contract).toMatch(new RegExp(`${FORMULA_MAX_LENGTH}\\s*символ|≤\\s*${FORMULA_MAX_LENGTH}`));
  });
});
