import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';

const sourcePath = path.resolve(import.meta.dirname, '../src/lib/client.ts');
const localRequire = createRequire(sourcePath);
const js = ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  fileName: sourcePath,
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true },
}).outputText;
const mod = { exports: {} };
const evaluate = new Function('require', 'module', 'exports', js);
evaluate(localRequire, mod, mod.exports);
const { normalizeErrorCode, normalizeErrorMessage, ApiRequestError } = mod.exports;

test('ApiRequestError captures status, code, message, and requestId', () => {
  const err = new ApiRequestError(403, 'FORBIDDEN', 'Access forbidden', 'req-12345');
  assert.equal(err.status, 403);
  assert.equal(err.code, 'FORBIDDEN');
  assert.equal(err.message, 'Access forbidden');
  assert.equal(err.requestId, 'req-12345');
  assert.equal(err.name, 'ApiRequestError');
  assert.ok(err instanceof Error);
});

test('HTTP status normalization maps 401, 403, 404, 409, 412, 428, 429, 502, 503 accurately', () => {
  const cases = [
    { status: 401, expectedCode: 'UNAUTHORIZED' },
    { status: 403, expectedCode: 'FORBIDDEN' },
    { status: 404, expectedCode: 'NOT_FOUND' },
    { status: 409, expectedCode: 'CONFLICT' },
    { status: 412, expectedCode: 'PRECONDITION_FAILED' },
    { status: 428, expectedCode: 'PRECONDITION_REQUIRED' },
    { status: 429, expectedCode: 'RATE_LIMITED' },
    { status: 502, expectedCode: 'BAD_GATEWAY' },
    { status: 503, expectedCode: 'SERVICE_UNAVAILABLE' },
    { status: 500, expectedCode: 'HTTP_500' },
  ];

  for (const c of cases) {
    assert.equal(normalizeErrorCode(c.status), c.expectedCode);
    const msg = normalizeErrorMessage(c.status);
    assert.ok(msg && typeof msg === 'string' && msg.length > 0);
  }
});
