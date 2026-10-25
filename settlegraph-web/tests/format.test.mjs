import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const sourcePath = path.resolve(import.meta.dirname, '../src/lib/format.ts');
const js = ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  fileName: sourcePath,
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText;
const mod = { exports: {} };
const evaluate = new Function('require', 'module', 'exports', js);
evaluate(null, mod, mod.exports);
const { formatMoney, shortDate, shorten } = mod.exports;

test('formatMoney handles null and undefined gracefully', () => {
  assert.equal(formatMoney(null), '—');
  assert.equal(formatMoney(undefined), '—');
});

test('formatMoney formats Ethiopian Birr strictly with BigInt integer division', () => {
  // 250000 minor units -> 2,500.00 ETB
  assert.equal(formatMoney({ minor_units: '250000', currency: 'ETB' }), '2,500.00 ETB');
  
  // 0 minor units -> 0.00 ETB
  assert.equal(formatMoney({ minor_units: '0', currency: 'ETB' }), '0.00 ETB');

  // 5 minor units -> 0.05 ETB (proper padding)
  assert.equal(formatMoney({ minor_units: '5', currency: 'ETB' }), '0.05 ETB');

  // 50 minor units -> 0.50 ETB
  assert.equal(formatMoney({ minor_units: '50', currency: 'ETB' }), '0.50 ETB');

  // 123456789 minor units -> 1,234,567.89 ETB
  assert.equal(formatMoney({ minor_units: '123456789', currency: 'ETB' }), '1,234,567.89 ETB');
});

test('formatMoney formats negative values without float conversion', () => {
  assert.equal(formatMoney({ minor_units: '-250000', currency: 'ETB' }), '−2,500.00 ETB');
  assert.equal(formatMoney({ minor_units: '-5', currency: 'ETB' }), '−0.05 ETB');
});

test('formatMoney handles very large values beyond JS Number.MAX_SAFE_INTEGER without precision loss', () => {
  // 900719925474099500 minor units is > 2^53 - 1
  const bigUnits = '900719925474099500';
  const formatted = formatMoney({ minor_units: bigUnits, currency: 'ETB' });
  assert.ok(formatted.endsWith('.00 ETB'));
  assert.ok(!formatted.includes('e+')); // No scientific notation float artifacts
  assert.equal(formatted, '9,007,199,254,740,995.00 ETB');
});

test('formatMoney handles non-numeric or empty string gracefully', () => {
  assert.equal(formatMoney({ minor_units: '', currency: 'ETB' }), '—');
  assert.equal(formatMoney({ minor_units: '   ', currency: 'ETB' }), '—');
});
