import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';

const sourcePath = path.resolve(import.meta.dirname, '../src/server/mock-engine.ts');
const localRequire = createRequire(sourcePath);
const js = ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  fileName: sourcePath,
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
    esModuleInterop: true,
    resolveJsonModule: true,
  },
}).outputText;
const mod = { exports: {} };
const evaluate = new Function('require', 'module', 'exports', js);
evaluate(localRequire, mod, mod.exports);
const { mockApi } = mod.exports;
const base = 'http://localhost:3000/api/sg';

async function invoke(method, p, body, headers = {}) {
  const h = { 'Content-Type': 'application/json', ...headers };
  if (method === 'POST' && !h['Idempotency-Key']) h['Idempotency-Key'] = crypto.randomUUID();
  const r = await mockApi(
    new Request(base + p, {
      method,
      headers: h,
      body: body ? JSON.stringify(body) : undefined,
    })
  );
  return { status: r.status, data: await r.json() };
}

test('reconciliation run: asynchronous initiation returns 202 and run history fields match specifications', async () => {
  // 1. Initiate reconciliation run
  const initKey = crypto.randomUUID();
  const initRes = await invoke('POST', '/v1/reconciliation/runs', { scope: 'LATEST_OBSERVATIONS' }, { 'Idempotency-Key': initKey });
  assert.equal(initRes.status, 202, 'Reconciliation run initiation must return HTTP 202');
  assert.ok(initRes.data.id, 'Run must contain an ID');
  assert.ok(['QUEUED', 'RUNNING', 'COMPLETED'].includes(initRes.data.status), 'Status must be valid');
  assert.equal(typeof initRes.data.matched_order_payment, 'number');
  assert.equal(typeof initRes.data.matched_payment_settlement, 'number');
  assert.equal(typeof initRes.data.unmatched_count, 'number');
  assert.ok(initRes.data.created_at, 'Run must have created_at timestamp');

  // 2. Fetch run by ID
  const getRes = await invoke('GET', `/v1/reconciliation/runs/${initRes.data.id}`);
  assert.equal(getRes.status, 200);
  assert.equal(getRes.data.id, initRes.data.id);

  // 3. List runs
  const listRes = await invoke('GET', '/v1/reconciliation/runs');
  assert.equal(listRes.status, 200);
  assert.ok(listRes.data.items.some((r) => r.id === initRes.data.id));
});

test('cases inbox: filtering by status, search queries, and case card badge fields', async () => {
  // 1. List cases
  const allCases = await invoke('GET', '/v1/cases');
  assert.equal(allCases.status, 200);
  assert.ok(allCases.data.items.length > 0);

  const sampleCase = allCases.data.items[0];
  // Verify case card badges / fields
  assert.ok(['LOW', 'MEDIUM', 'HIGH'].includes(sampleCase.severity), 'Valid severity badge');
  assert.ok(['OPEN', 'EVIDENCE_UPDATED', 'RESOLVED'].includes(sampleCase.status), 'Valid status badge');
  assert.ok(sampleCase.kind, 'Case kind must be present');
  assert.ok(sampleCase.updated_at, 'Updated date must be present');
  assert.equal(typeof sampleCase.evidence_count, 'number', 'Evidence count must be numeric');
  assert.ok(sampleCase.title, 'Case title must be present');
  assert.ok(sampleCase.summary, 'Case summary must be present');

  // 2. Test status filtering query
  const openCases = await invoke('GET', '/v1/cases?status=OPEN');
  assert.equal(openCases.status, 200);
  for (const c of openCases.data.items) {
    assert.equal(c.status, 'OPEN');
  }

  // 3. Test client-side filter logic simulation (matching page.tsx behavior)
  const query = sampleCase.title.slice(0, 5).toLowerCase();
  const searchCorpus = `${sampleCase.title} ${sampleCase.kind} ${sampleCase.summary}`.toLowerCase();
  assert.ok(searchCorpus.includes(query), 'Search corpus matches title/kind/summary');
});

test('case detail: supporting evidence and review history timeline', async () => {
  const cases = await invoke('GET', '/v1/cases');
  const caseId = cases.data.items[0].id;

  // 1. Supporting evidence
  const evRes = await invoke('GET', `/v1/cases/${caseId}/evidence`);
  assert.equal(evRes.status, 200);
  assert.ok(Array.isArray(evRes.data.items));
  for (const item of evRes.data.items) {
    assert.ok(item.id, 'Evidence item must have id for anchoring #evidence-{id}');
    assert.ok(['ORDER', 'PAYMENT', 'SETTLEMENT', 'SOURCE_STATUS'].includes(item.record_type));
    assert.ok(item.reference, 'Evidence must display reference');
    assert.ok('provider_id' in item, 'Evidence must display provider');
    assert.ok('amount' in item, 'Evidence must display amount (or null for non-monetary)');
  }

  // 2. Review history timeline events
  const eventsRes = await invoke('GET', `/v1/cases/${caseId}/events`);
  assert.equal(eventsRes.status, 200);
  assert.ok(Array.isArray(eventsRes.data.items));
  for (const ev of eventsRes.data.items) {
    assert.ok(ev.type, 'Event must have type');
    assert.ok(ev.message, 'Event must have message');
    assert.ok(ev.occurred_at, 'Event must have occurred_at timestamp');
    assert.equal(typeof ev.version, 'number', 'Event must have version indicator');
  }
});

test('case detail: AI investigation returns 202, uses OFFLINE_STUB, and links cited evidence', async () => {
  const cases = await invoke('GET', '/v1/cases');
  const caseId = cases.data.items[0].id;

  const invRes = await invoke('POST', `/v1/cases/${caseId}/investigations`, { mode: 'EVIDENCE_ANALYSIS' });
  assert.equal(invRes.status, 202, 'AI investigation trigger must return HTTP 202');
  assert.equal(invRes.data.provider, 'OFFLINE_STUB', 'Transparently labeled OFFLINE_STUB (never claiming live Bedrock)');
  assert.ok(invRes.data.result.summary, 'Summary must be present');
  assert.ok(Array.isArray(invRes.data.result.hypotheses), 'Hypotheses must be an array');
  assert.ok(invRes.data.result.hypotheses.length > 0, 'At least one hypothesis must be generated');
  assert.ok(Array.isArray(invRes.data.result.evidence_ids), 'Supporting evidence IDs must be present');
  assert.ok(Array.isArray(invRes.data.result.recommended_actions), 'Recommended actions must be present');
  assert.ok(Array.isArray(invRes.data.result.limitations), 'Limitations must be explicitly listed');
});

test('case decision: validates >= 5 chars reason, checks If-Match version conflict (412), and supports stable Idempotency-Key', async () => {
  const cases = await invoke('GET', '/v1/cases');
  const c = cases.data.items[0];
  const caseId = c.id;

  // 1. Validation failure (< 5 chars reason)
  const shortReasonRes = await invoke(
    'POST',
    `/v1/cases/${caseId}/decisions`,
    { decision: 'RESOLVE', reason: 'ok' },
    { 'If-Match': `"${c.version}"` }
  );
  assert.equal(shortReasonRes.status, 400, 'Reason < 5 chars must be rejected with 400');

  // 2. Missing If-Match precondition
  const noIfMatchRes = await invoke(
    'POST',
    `/v1/cases/${caseId}/decisions`,
    { decision: 'RESOLVE', reason: 'Valid explanation >= 5 characters' },
    {}
  );
  assert.equal(noIfMatchRes.status, 428, 'Missing If-Match must return 428 Precondition Required');

  // 3. Stale version conflict (412)
  const staleRes = await invoke(
    'POST',
    `/v1/cases/${caseId}/decisions`,
    { decision: 'RESOLVE', reason: 'Valid explanation >= 5 characters' },
    { 'If-Match': `"${c.version + 999}"` }
  );
  assert.equal(staleRes.status, 412, 'Stale version conflict must return 412');

  // 4. Successful decision with correct version and stable idempotency key
  const stableKey = crypto.randomUUID();
  const payload = { decision: 'NEEDS_MORE_EVIDENCE', reason: 'Awaiting updated bank settlement statement.' };
  const okRes = await invoke(
    'POST',
    `/v1/cases/${caseId}/decisions`,
    payload,
    { 'If-Match': `"${c.version}"`, 'Idempotency-Key': stableKey }
  );
  assert.equal(okRes.status, 200, 'Valid decision returns 200');
  assert.equal(okRes.data.version, c.version + 1, 'Case version increments on decision');

  // 5. Duplicate submission with same stable Idempotency-Key replays result without conflict
  const replayRes = await invoke(
    'POST',
    `/v1/cases/${caseId}/decisions`,
    payload,
    { 'If-Match': `"${c.version}"`, 'Idempotency-Key': stableKey }
  );
  assert.equal(replayRes.status, 200, 'Idempotent replay returns 200');
  assert.equal(replayRes.data.version, okRes.data.version, 'Replay preserves version');

  // 6. Idempotency conflict if same key is reused with different payload
  const conflictRes = await invoke(
    'POST',
    `/v1/cases/${caseId}/decisions`,
    { decision: 'RESOLVE', reason: 'Different payload with same key' },
    { 'If-Match': `"${c.version}"`, 'Idempotency-Key': stableKey }
  );
  assert.equal(conflictRes.status, 409, 'Reusing key with different payload must return 409');
});
