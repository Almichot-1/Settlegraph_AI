import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';

// Transpile this server-only mock using the already installed TypeScript dependency.
// No Next.js server or live bank dependency is involved.
const sourcePath=path.resolve(import.meta.dirname,'../src/server/mock-engine.ts');
const localRequire=createRequire(sourcePath);
const js=ts.transpileModule(fs.readFileSync(sourcePath,'utf8'),{
 fileName:sourcePath,compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022,esModuleInterop:true,resolveJsonModule:true}
}).outputText;
const mod={exports:{}};
const evaluate=new Function('require','module','exports',js);
evaluate(localRequire,mod,mod.exports);
const {mockApi}=mod.exports;
const base='http://localhost:3000/api/sg';
async function invoke(method,p,body,headers={}){
 const h={'Content-Type':'application/json',...headers};
 if(method==='POST'&&!h['Idempotency-Key'])h['Idempotency-Key']=crypto.randomUUID();
 const r=await mockApi(new Request(base+p,{method,headers:h,body:body?JSON.stringify(body):undefined}));
 return {status:r.status,data:await r.json()};
}
test('dashboard amounts remain decimal-string ETB and stages separate',async()=>{
 const d=await invoke('GET','/v1/dashboard/summary');assert.equal(d.status,200);
 for(const k of ['orders_expected','payments_accepted','settlements_observed','unmatched_payments']){
  assert.match(d.data[k].minor_units,/^-?\d+$/);assert.equal(d.data[k].currency,'ETB');
 }
 assert.ok('source_status' in d.data.coverage);
});
test('scenario lifecycle changes observations and creates evidence cases',async()=>{
 const s=await invoke('POST','/v1/demo/scenarios/delayed_settlement/runs',{seed:102});assert.equal(s.status,201);
 const id=s.data.id;
 for(let step=0;step<3;step++){
  const a=await invoke('POST',`/v1/demo/scenario-runs/${id}/advance`,{expected_step:step});
  assert.equal(a.status,200);assert.equal(a.data.step,step+1);
 }
 const cases=await invoke('GET',`/v1/cases?scenario_run_id=${id}`);assert.equal(cases.data.items.length,1);
 assert.equal(cases.data.items[0].status,'EVIDENCE_UPDATED');
 const tx=await invoke('GET',`/v1/transactions?scenario_run_id=${id}`);assert.equal(tx.data.items.length,1);
 const settlement=await invoke('GET','/v1/settlements');assert.ok(settlement.data.items.some(x=>x.scenario_run_id===id));
});
test('case decision requires version and idempotency',async()=>{
 const cases=await invoke('GET','/v1/cases');const c=cases.data.items[0];
 const id=c.id;
 const noVersion=await invoke('POST',`/v1/cases/${id}/decisions`,{decision:'NEEDS_MORE_EVIDENCE',reason:'Require a bank settlement'},{});
 assert.equal(noVersion.status,428);
 const key=crypto.randomUUID(),payload={decision:'NEEDS_MORE_EVIDENCE',reason:'Require a bank settlement'};
 const ok=await invoke('POST',`/v1/cases/${id}/decisions`,payload,{'If-Match':`"${c.version}"`,'Idempotency-Key':key});assert.equal(ok.status,200);
 const duplicate=await invoke('POST',`/v1/cases/${id}/decisions`,payload,{'If-Match':`"${c.version}"`,'Idempotency-Key':key});assert.equal(duplicate.status,200);
 assert.equal(duplicate.data.version,ok.data.version);
 const stale=await invoke('POST',`/v1/cases/${id}/decisions`,payload,{'If-Match':`"${c.version}"`});assert.equal(stale.status,412);
});
test('mock investigation uses OFFLINE_STUB and provides referenced evidence',async()=>{
 const cases=await invoke('GET','/v1/cases');const c=cases.data.items[0];
 const r=await invoke('POST',`/v1/cases/${c.id}/investigations`,{mode:'EVIDENCE_ANALYSIS'});
 assert.equal(r.status,202);assert.equal(r.data.provider,'OFFLINE_STUB');assert.ok(r.data.result.evidence_ids.length>0);
});
test('idempotency key collision with different body is rejected',async()=>{
 const key=crypto.randomUUID(),p='/v1/demo/scenarios/normal_flow/runs';
 const first=await invoke('POST',p,{seed:101},{'Idempotency-Key':key});assert.equal(first.status,201);
 const second=await invoke('POST',p,{seed:999},{'Idempotency-Key':key});assert.equal(second.status,409);
});
test('provider outage scenario simulates source gap, coverage degradation, and recovery',async()=>{
 const s=await invoke('POST','/v1/demo/scenarios/provider_outage/runs',{seed:104});assert.equal(s.status,201);
 const id=s.data.id;
 assert.equal(s.data.scenario_key,'provider_outage');
 assert.equal(s.data.total_steps,3);

 const step1=await invoke('POST',`/v1/demo/scenario-runs/${id}/advance`,{expected_step:0});
 assert.equal(step1.status,200);assert.equal(step1.data.step,1);

 const step2=await invoke('POST',`/v1/demo/scenario-runs/${id}/advance`,{expected_step:1});
 assert.equal(step2.status,200);assert.equal(step2.data.step,2);

 const d1=await invoke('GET','/v1/dashboard/summary');
 assert.equal(d1.data.coverage.source_status,'INCOMPLETE_SOURCE');
 assert.equal(d1.data.coverage.is_complete,false);
 const bankB=d1.data.providers.find(p=>p.id==='BANK_B');
 assert.equal(bankB.sync_status,'OUTAGE');

 const outageCases=await invoke('GET',`/v1/cases?scenario_run_id=${id}`);
 assert.equal(outageCases.data.items.length,1);
 const oc=outageCases.data.items[0];
 assert.equal(oc.kind,'SOURCE_UNAVAILABLE');

 const inv=await invoke('POST',`/v1/cases/${oc.id}/investigations`,{mode:'EVIDENCE_ANALYSIS'});
 assert.equal(inv.status,202);
 assert.ok(inv.data.result.hypotheses[0].explanation.includes('Provider records are incomplete'));

 const step3=await invoke('POST',`/v1/demo/scenario-runs/${id}/advance`,{expected_step:2});
 assert.equal(step3.status,200);assert.equal(step3.data.step,3);
 assert.equal(step3.data.status,'COMPLETED');

 const d2=await invoke('GET','/v1/dashboard/summary');
 assert.equal(d2.data.coverage.source_status,'COMPLETE');
 assert.equal(d2.data.coverage.is_complete,true);

 const updatedCases=await invoke('GET',`/v1/cases?scenario_run_id=${id}`);
 assert.equal(updatedCases.data.items[0].status,'EVIDENCE_UPDATED');

 const auditRes=await invoke('GET','/v1/audit-events');
 assert.ok(auditRes.data.items.length>0);
 const auditItem=auditRes.data.items[0];
 assert.ok(auditItem.occurred_at&&auditItem.actor&&auditItem.action&&auditItem.resource_type&&auditItem.resource_id);
});
