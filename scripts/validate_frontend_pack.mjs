import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
const base = path.resolve(import.meta.dirname, '..');
const manifest = JSON.parse(fs.readFileSync(path.join(base,'docs','OPERATION_MANIFEST.json'),'utf8'));
const generated = fs.readFileSync(path.join(base,'settlegraph-web','src','generated','operation-manifest.ts'),'utf8');
const client = fs.readFileSync(path.join(base,'settlegraph-web','src','lib','api.ts'),'utf8');
const names = new Set(manifest.map(x=>x.operationId));
assert.equal(manifest.length,39,'Expected 39 public API operations');
assert.equal(names.size,39,'No duplicate operation IDs allowed');
for (const op of manifest) {
  assert.ok(generated.includes(`"operationId": "${op.operationId}"`),`Missing generated operation ${op.operationId}`);
  if (op.operationId==='createDemoSession'||op.operationId==='acceptBankSimEvent') continue;
  assert.ok(client.includes(op.operationId+': '),`Missing typed client wrapper for ${op.operationId}`);
}
for (const item of ['demo-session-response','dashboard-summary','case-open','case-evidence','scenario-run']) {
 const mock=JSON.parse(fs.readFileSync(path.join(base,'fixtures',item+'.json'),'utf8'));
 assert.ok(mock && typeof mock==='object',`Broken fixture ${item}`);
}
const money=JSON.parse(fs.readFileSync(path.join(base,'fixtures','dashboard-summary.json'),'utf8'));
for (const field of ['orders_expected','payments_accepted','settlements_observed','unmatched_payments']){
 assert.match(money[field].minor_units,/^-?[0-9]+$/);
 assert.equal(money[field].currency,'ETB');
}
console.log('PASS: 39-operation manifest, 37 typed browser wrappers, fixture sanity and safe money strings.');
