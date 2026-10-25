import membersFixture from './members.json';
/** In-memory DEMO ONLY. The backend owns real business logic. This mock never claims live bank data. */
import type * as T from '@/generated/types';
import demoSession from './demo-session-response.json';
import initialDashboard from './dashboard-summary.json';
import initialCase from './case-open.json';
import initialEvidence from './case-evidence.json';
import validAI from './ai-investigation-valid.json';
import catalog from './scenario-catalog.json';

const orgId = demoSession.default_organization_id;
const meId = demoSession.user.id;
const asMoney = (minor: bigint | number | string): T.Money => ({minor_units: minor.toString(),currency:'ETB'});
const sum = (values: T.Money[]): T.Money => asMoney(values.reduce((total, item) => total + BigInt(item.minor_units), 0n));
const iso = () => new Date().toISOString();
const uuid = () => crypto.randomUUID();
const baseTime = '2026-10-24T09:00:00Z';
const defaultScenarioId = '22222222-2222-4222-8222-222222222222';

const seedTransactions: T.Transaction[] = [
  {id:'eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01',provider_id:'BANK_A',external_reference:'SIM-TRF-1045',order_reference:'ORD-1045',amount:asMoney(250000),state:'ACCEPTED',occurred_at:'2026-10-24T09:00:00Z',observed_at:'2026-10-24T09:00:05Z',source_attestation:'SIGNED_SIMULATED_PROVIDER',scenario_run_id:defaultScenarioId},
  {id:'eeeeeeee-eeee-4eee-8eee-eeeeeeeeee02',provider_id:'BANK_B',external_reference:'BANKB-TX-988',order_reference:'ORD-1044',amount:asMoney(142000),state:'ACCEPTED',occurred_at:'2026-10-24T08:47:00Z',observed_at:'2026-10-24T08:47:01Z',source_attestation:'SIGNED_SIMULATED_PROVIDER',scenario_run_id:null},
  {id:'eeeeeeee-eeee-4eee-8eee-eeeeeeeeee03',provider_id:'WALLET_A',external_reference:'WLT-TX-114',order_reference:'ORD-1043',amount:asMoney(87500),state:'ACCEPTED',occurred_at:'2026-10-24T08:10:00Z',observed_at:'2026-10-24T08:10:03Z',source_attestation:'SIGNED_SIMULATED_PROVIDER',scenario_run_id:null},
  {id:'eeeeeeee-eeee-4eee-8eee-eeeeeeeeee04',provider_id:'BANK_A',external_reference:'SIM-TRF-1042',order_reference:'ORD-1042',amount:asMoney(50000),state:'REFUNDED',occurred_at:'2026-10-24T07:50:00Z',observed_at:'2026-10-24T07:50:04Z',source_attestation:'SIGNED_SIMULATED_PROVIDER',scenario_run_id:null},
];
const seedSettlements:T.Settlement[] = [
  {id:'ffffffff-ffff-4fff-8fff-ffffffffff01',provider_id:'BANK_B',external_reference:'SETTLE-988',payment_reference:'BANKB-TX-988',amount:asMoney(142000),occurred_at:'2026-10-24T08:55:00Z',observed_at:'2026-10-24T08:55:03Z',scenario_run_id:null},
  {id:'ffffffff-ffff-4fff-8fff-ffffffffff02',provider_id:'WALLET_A',external_reference:'SETTLE-114',payment_reference:'WLT-TX-114',amount:asMoney(87500),occurred_at:'2026-10-24T08:22:00Z',observed_at:'2026-10-24T08:22:02Z',scenario_run_id:null},
];
const seedOrders:T.Order[] = seedTransactions.map((t,i)=>({id:`dddddddd-dddd-4ddd-8ddd-${String(i+1).padStart(12,'0')}`,external_reference:t.order_reference??`ORD-${1045-i}`,amount:t.amount,status:t.state==='ACCEPTED'?'PAYMENT_OBSERVED':'OPEN',created_at:t.occurred_at,scenario_run_id:t.scenario_run_id}));
const seededCase = initialCase as T.Case;
const seedEvents:T.CaseEvent[] = [
 {id:'77777777-7777-4777-8777-777777777771',case_id:seededCase.id,type:'OPENED',occurred_at:seededCase.created_at,actor:'system',message:'No matching settlement found after the configured grace period.',version:1},
 {id:'77777777-7777-4777-8777-777777777772',case_id:seededCase.id,type:'EVIDENCE_ADDED',occurred_at:seededCase.updated_at,actor:'worker',message:'Provider source coverage confirmed through the observation watermark.',version:2},
];
type Store = {
 transactions:T.Transaction[]; settlements:T.Settlement[]; orders:T.Order[]; cases:T.Case[];
 evidence:Record<string,T.Evidence[]>; events:Record<string,T.CaseEvent[]>;
 investigations:Record<string,T.Investigation[]>; runs:T.ReconciliationRun[];
 matches:Record<string,T.Match[]>; proofs:Record<string,T.MatchProof>;
 scenarioRuns:T.ScenarioRun[]; providers:T.Provider[];
 idempotency:Record<string,{status:number; data:unknown; signature:string}>; audit:T.AuditEvent[];
};
function generateMatchesForRun(s:Store, r:T.ReconciliationRun): T.Match[] {
 const matches: T.Match[] = [];
 for(const txn of s.transactions.filter(t=>t.state==='ACCEPTED')){
   const order = s.orders.find(o=>o.external_reference===txn.order_reference);
   if(order){
     const proofId = uuid(), matchId = uuid();
     matches.push({
       id: matchId,
       run_id: r.id,
       stage: 'ORDER_PAYMENT',
       left_evidence_id: order.id,
       right_evidence_id: txn.id,
       allocated_amount: txn.amount,
       rule: 'EXACT_ORDER_REFERENCE_AND_AMOUNT',
       proof_id: proofId,
     });
     s.proofs[proofId] = {
       id: proofId,
       run_id: r.id,
       rule_version: r.rules_version,
       stage: 'ORDER_PAYMENT',
       checks: [
         'EXACT_ORDER_REFERENCE_MATCH',
         'CURRENCY_ETB_MATCH',
         'EXACT_AMOUNT_MATCH',
         'SIGNED_PROVIDER_WATERMARK_VALID'
       ],
       source_evidence_ids: [order.id, txn.id],
       result: 'MATCHED_IN_IMPORTED_RECORDS',
     };
   }
 }
 for(const stl of s.settlements){
   const txn = s.transactions.find(t=>t.external_reference===stl.payment_reference);
   if(txn){
     const proofId = uuid(), matchId = uuid();
     matches.push({
       id: matchId,
       run_id: r.id,
       stage: 'PAYMENT_SETTLEMENT',
       left_evidence_id: txn.id,
       right_evidence_id: stl.id,
       allocated_amount: stl.amount,
       rule: 'EXACT_PROVIDER_REFERENCE_AND_AMOUNT',
       proof_id: proofId,
     });
     s.proofs[proofId] = {
       id: proofId,
       run_id: r.id,
       rule_version: r.rules_version,
       stage: 'PAYMENT_SETTLEMENT',
       checks: [
         'PROVIDER_ID_MATCH',
         'PAYMENT_REFERENCE_MATCH',
         'EXACT_SETTLEMENT_AMOUNT_MATCH',
         'SOURCE_SETTLEMENT_BATCH_CONFIRMED'
       ],
       source_evidence_ids: [txn.id, stl.id],
       result: 'MATCHED_IN_IMPORTED_RECORDS',
     };
   }
 }
 return matches;
}
const createStore=(): Store => {
 const seedRunId = '11111111-1111-4111-8111-111111111110';
 const seedRun: T.ReconciliationRun = {
   id: seedRunId,
   scope: 'LATEST_OBSERVATIONS',
   status: 'COMPLETED',
   rules_version: 'v1',
   created_at: '2026-10-24T08:58:00Z',
   completed_at: '2026-10-24T08:58:02Z',
   matched_order_payment: 3,
   matched_payment_settlement: 2,
   unmatched_count: 1,
   scenario_run_id: null,
 };
 const s: Store = {
  transactions:[...seedTransactions],settlements:[...seedSettlements],orders:[...seedOrders],cases:[seededCase],
  evidence:{[seededCase.id]:initialEvidence.items as T.Evidence[]},events:{[seededCase.id]:seedEvents}, investigations:{},
  runs:[seedRun],matches:{},proofs:{},scenarioRuns:[],idempotency:{},providers:[...(initialDashboard as T.DashboardSummary).providers,{id:'WALLET_A',name:'Simulated Wallet',kind:'WALLET',sync_status:'HEALTHY',last_observed_at:baseTime}],audit:[],
 };
 s.matches[seedRunId] = generateMatchesForRun(s, seedRun);
 return s;
};
const globalState=globalThis as typeof globalThis & {__sgMockStore?:Store};
const state=()=>globalState.__sgMockStore ?? (globalState.__sgMockStore=createStore());
const output=(status:number,data:unknown) => Response.json(data,{status,headers:{'cache-control':'no-store'}});
const error=(status:number,code:string,message:string) => output(status,{error:{code,message,request_id:'mock-'+uuid()}});
function paginate<T>(items:T[],url:URL):{items:T[];next_cursor:string|null}{
 const cursor=Math.max(0,Number(url.searchParams.get('cursor') ?? 0) || 0);
 const limit=Math.max(1,Math.min(100,Number(url.searchParams.get('limit') ?? 50)||50));
 return {items:items.slice(cursor,cursor+limit),next_cursor:cursor+limit<items.length?String(cursor+limit):null};
}
function audit(s:Store,action:string,type:string,id:string){s.audit.unshift({id:uuid(),actor:'Demo Owner',action,resource_type:type,resource_id:id,occurred_at:iso()});}
function addEvent(s:Store,c:T.Case,type:T.CaseEvent['type'],message:string){
 const ev:T.CaseEvent={id:uuid(),case_id:c.id,type,occurred_at:iso(),actor:'Demo Owner',message,version:c.version};
 (s.events[c.id]??=[]).unshift(ev);
}
function appendPayment(s:Store, run:T.ScenarioRun){
 const ref=`SIM-${run.scenario_key.toUpperCase()}-${run.id.slice(0,8)}`;
 const id=uuid();
 s.transactions.unshift({id,provider_id:'BANK_A',external_reference:ref,order_reference:`ORD-${run.id.slice(0,8)}`,amount:asMoney(250000),state:'ACCEPTED',occurred_at:iso(),observed_at:iso(),source_attestation:'SIGNED_SIMULATED_PROVIDER',scenario_run_id:run.id});
 s.orders.unshift({id:uuid(),external_reference:`ORD-${run.id.slice(0,8)}`,amount:asMoney(250000),status:'PAYMENT_OBSERVED',created_at:iso(),scenario_run_id:run.id});
}
function appendSettlement(s:Store,run:T.ScenarioRun){
 const txn=s.transactions.find(t=>t.scenario_run_id===run.id);
 if(!txn) return;
 if(!s.settlements.some(t=>t.payment_reference===txn.external_reference))
 s.settlements.unshift({id:uuid(),provider_id:'BANK_B',external_reference:`STL-${run.id.slice(0,8)}`,payment_reference:txn.external_reference,amount:txn.amount,occurred_at:iso(),observed_at:iso(),scenario_run_id:run.id});
 const c=s.cases.find(c=>c.scenario_run_id===run.id && c.status!=='RESOLVED');
 if(c){c.status='EVIDENCE_UPDATED';c.version+=1;c.updated_at=iso();
   (s.evidence[c.id]??=[]).push({id:uuid(),record_type:'SETTLEMENT',reference:`STL-${run.id.slice(0,8)}`,amount:txn.amount,occurred_at:iso(),provider_id:'BANK_B'});
   c.evidence_count=s.evidence[c.id].length;addEvent(s,c,'EVIDENCE_ADDED','Additional simulated bank settlement observed after recovery.');}
}
function openCase(s:Store,run:T.ScenarioRun, kind:T.Case['kind']){
 const txn=s.transactions.find(t=>t.scenario_run_id===run.id);
 if(!txn || s.cases.some(c=>c.scenario_run_id===run.id))return;
 const c:T.Case={id:uuid(),kind,status:'OPEN',severity:'MEDIUM',title:kind==='SOURCE_UNAVAILABLE'?'Bank B source temporarily unavailable':`Unmatched settlement for ${txn.external_reference}`,
 summary:kind==='SOURCE_UNAVAILABLE'?'Bank B data is unavailable. No financial conclusion can be reached until its records return.':'Payment reported accepted; matching settlement not observed within the covered source window.',version:1,created_at:iso(),updated_at:iso(),evidence_count:2,assigned_to:meId,scenario_run_id:run.id};
 s.cases.unshift(c);
 s.evidence[c.id]=[{id:uuid(),record_type:'PAYMENT',reference:txn.external_reference,amount:txn.amount,occurred_at:txn.occurred_at,provider_id:txn.provider_id},
  {id:uuid(),record_type:'SOURCE_STATUS',reference:kind==='SOURCE_UNAVAILABLE'?'BANK_B:unavailable':'BANK_B:complete-window',amount:null,occurred_at:iso(),provider_id:'BANK_B'}];
 s.events[c.id]=[];addEvent(s,c,'OPENED',c.summary);audit(s,'CASE_CREATED','CASE',c.id);
}
function advance(s:Store,run:T.ScenarioRun) {
 run.step+=1;run.status=run.step>=run.total_steps?'COMPLETED':'RUNNING';run.virtual_now=iso();
 if(run.step===1)appendPayment(s,run);
 if(run.scenario_key==='delayed_settlement' && run.step===2)openCase(s,run,'SETTLEMENT_NOT_OBSERVED');
 if(run.scenario_key==='provider_outage' && run.step===2){openCase(s,run,'SOURCE_UNAVAILABLE');s.providers=s.providers.map(p=>p.id==='BANK_B'?{...p,sync_status:'OUTAGE'}:p);}
 if(run.step===run.total_steps){appendSettlement(s,run);s.providers=s.providers.map(p=>p.id==='BANK_B'?{...p,sync_status:'HEALTHY',last_observed_at:iso()}:p);}
 audit(s,'SCENARIO_STEP_ADVANCED','SCENARIO_RUN',run.id);
}

/** Backend-shaped responses for browser development. Not a banking simulator or backend replacement. */
export async function mockApi(req:Request):Promise<Response>{
 const url=new URL(req.url),p=url.pathname.replace(/^\/api\/sg/,'');
 const method=req.method.toUpperCase(),s=state(),seg=p.split('/').filter(Boolean);
 const body:Record<string,unknown>=(method==='POST' ? await req.json().catch(()=>({})) : {}) as Record<string,unknown>;
 const idem=req.headers.get('idempotency-key');
 if(method==='POST' && !idem)return error(400,'IDEMPOTENCY_REQUIRED','Idempotency-Key is required');
 const idemKey=`${method}:${p}:${idem}`;
 const signature=JSON.stringify(body);
 if(method==='POST' && s.idempotency[idemKey]){
  const previous=s.idempotency[idemKey];
  if(previous.signature!==signature)return error(409,'IDEMPOTENCY_CONFLICT','This idempotency key was already used with a different request payload.');
  return output(previous.status,previous.data);
 }
 const reply=(status:number,data:unknown)=>{if(method==='POST')s.idempotency[idemKey]={status,data,signature};return output(status,data);};
 if(method==='GET' && p==='/health/live')return output(200,{status:'UP',service:'settlegraph-web-mock'});
 if(method==='GET' && p==='/health/ready')return output(200,{status:'UP',service:'settlegraph-web-mock'});
 if(method==='GET' && p==='/v1/me')return output(200,demoSession.user);
 if(method==='GET' && p==='/v1/providers')return output(200,paginate(s.providers,url));
 if(method==='GET' && p==='/v1/members')return output(200,paginate(membersFixture.items,url));
 if(method==='GET' && p==='/v1/accounts')return output(200,{items:[],next_cursor:null});
 if(method==='GET' && p==='/v1/dashboard/summary'){
  const payments=s.transactions.filter(t=>t.state==='ACCEPTED');const settlements=s.settlements;
  const unmatched=payments.filter(t=>!settlements.some(x=>x.payment_reference===t.external_reference));
  const dashboard:T.DashboardSummary={as_of:iso(),orders_expected:sum(s.orders.map(o=>o.amount)),payments_accepted:sum(payments.map(t=>t.amount)),
   settlements_observed:sum(settlements.map(t=>t.amount)),unmatched_payments:sum(unmatched.map(t=>t.amount)),open_cases:s.cases.filter(c=>c.status!=='RESOLVED').length,
   providers:s.providers,coverage:{source_status:s.providers.some(p=>p.sync_status==='OUTAGE')?'INCOMPLETE_SOURCE':'COMPLETE',is_complete:!s.providers.some(p=>p.sync_status==='OUTAGE')}};
  return output(200,dashboard);
 }
 if(method==='GET' && p==='/v1/dashboard/series'){
  if(!url.searchParams.get('from')||!url.searchParams.get('to'))return error(400,'VALIDATION_ERROR','from and to are required');
  return output(200,{granularity:url.searchParams.get('granularity')==='HOUR'?'HOUR':'DAY',points:Array.from({length:7},(_,i)=>({bucket_start:new Date(Date.now()-(6-i)*86400_000).toISOString(),payments_accepted:asMoney((i+2)*90000),settlements_observed:asMoney((i+1)*81000)}))});
 }
 if(method==='GET' && p==='/v1/orders')return output(200,paginate(s.orders,url));
 if(method==='GET' && seg[1]==='orders' && seg.length===3){const x=s.orders.find(x=>x.id===seg[2]);return x?output(200,x):error(404,'NOT_FOUND','Order not found');}
 if(method==='GET' && p==='/v1/transactions')return output(200,paginate(s.transactions.filter(x=>(!url.searchParams.get('provider_id')||x.provider_id===url.searchParams.get('provider_id')) && (!url.searchParams.get('scenario_run_id')||x.scenario_run_id===url.searchParams.get('scenario_run_id'))),url));
 if(method==='GET' && seg[1]==='transactions' && seg.length===3){const x=s.transactions.find(x=>x.id===seg[2]);return x?output(200,x):error(404,'NOT_FOUND','Transaction not found');}
 if(method==='GET' && p==='/v1/settlements')return output(200,paginate(s.settlements,url));
 if(method==='GET' && seg[1]==='settlements' && seg.length===3){const x=s.settlements.find(x=>x.id===seg[2]);return x?output(200,x):error(404,'NOT_FOUND','Settlement not found');}
 if(method==='GET' && p==='/v1/reconciliation/runs')return output(200,paginate(s.runs,url));
 if(method==='POST' && p==='/v1/reconciliation/runs'){
  const r:T.ReconciliationRun={id:uuid(),scope:body.scope==='SCENARIO_RUN'?'SCENARIO_RUN':'LATEST_OBSERVATIONS',status:'COMPLETED',rules_version:'v1',created_at:iso(),completed_at:iso(),
   matched_order_payment:s.transactions.filter(t=>t.state==='ACCEPTED').length,matched_payment_settlement:s.settlements.length,unmatched_count:s.transactions.filter(t=>t.state==='ACCEPTED'&&!s.settlements.some(x=>x.payment_reference===t.external_reference)).length,
   scenario_run_id:typeof body.scenario_run_id==='string'?body.scenario_run_id:null};
  s.runs.unshift(r);
  s.matches[r.id] = generateMatchesForRun(s, r);
  audit(s,'RECONCILIATION_STARTED','RECONCILIATION_RUN',r.id);return reply(202,r);
 }
 if(method==='GET' && seg[1]==='reconciliation' && seg[2]==='runs' && seg.length===4){const r=s.runs.find(x=>x.id===seg[3]);return r?output(200,r):error(404,'NOT_FOUND','Run not found');}
 if(method==='GET' && seg[1]==='reconciliation' && seg[2]==='runs' && seg.length===5 && seg[4]==='matches')return output(200,paginate(s.matches[seg[3]]??[],url));
 if(method==='GET' && seg[1]==='reconciliation' && seg[2]==='proofs' && seg.length===4){const pr=s.proofs[seg[3]];return pr?output(200,pr):error(404,'NOT_FOUND','Match proof not found');}
 if(method==='GET' && p==='/v1/cases')return output(200,paginate(s.cases.filter(x=>(!url.searchParams.get('status')||x.status===url.searchParams.get('status'))&&(!url.searchParams.get('assigned_to')||x.assigned_to===url.searchParams.get('assigned_to'))&&(!url.searchParams.get('scenario_run_id')||x.scenario_run_id===url.searchParams.get('scenario_run_id'))),url));
 if(seg[1]==='cases' && seg[2]){
   const c=s.cases.find(x=>x.id===seg[2]);if(!c)return error(404,'NOT_FOUND','Case not found');
   if(method==='GET' && seg.length===3)return output(200,c);
   if(method==='GET' && seg[3]==='evidence')return output(200,paginate(s.evidence[c.id]??[],url));
   if(method==='GET' && seg[3]==='events')return output(200,paginate(s.events[c.id]??[],url));
   if(method==='GET' && seg[3]==='investigations' && seg.length===4)return output(200,paginate(s.investigations[c.id]??[],url));
   if(method==='GET' && seg[3]==='investigations' && seg.length===5){const v=(s.investigations[c.id]??[]).find(x=>x.id===seg[4]);return v?output(200,v):error(404,'NOT_FOUND','Investigation not found');}
   if(method==='POST' && seg[3]==='investigations'){
      const evidence=s.evidence[c.id]??[];
      const result:T.InvestigationResult={...(validAI as T.InvestigationResult),evidence_ids:evidence.map(x=>x.id),hypotheses:[{explanation:c.kind==='SOURCE_UNAVAILABLE'?'Provider records are incomplete; a missing transfer cannot be inferred.':'The simulated bank settlement may be delayed; verify it against the next source watermark.',evidence_ids:evidence.map(x=>x.id)}]};
      const ai:T.Investigation={id:uuid(),case_id:c.id,status:'COMPLETED',provider:'OFFLINE_STUB',created_at:iso(),completed_at:iso(),evidence_snapshot_hash:'mock-local-fixture',result};
      (s.investigations[c.id]??=[]).unshift(ai);addEvent(s,c,'INVESTIGATION_COMPLETED','Offline mock investigation completed (NOT an actual Bedrock call).');audit(s,'INVESTIGATION_CREATED','CASE',c.id);return reply(202,ai);
   }
   if(method==='POST' && ['assign','decisions'].includes(seg[3]??'')){
      const header=req.headers.get('if-match');if(!header)return error(428,'PRECONDITION_REQUIRED','If-Match required');
      if(header!==`"${c.version}"`)return error(412,'VERSION_CONFLICT','This case changed; refresh before retrying');
      if(seg[3]==='assign') {c.assigned_to=String(body.assignee_id??'');c.version+=1;addEvent(s,c,'ASSIGNED','Case assigned to reviewer.');}
      else {const decision=String(body.decision??'');if(String(body.reason??'').length<5)return error(400,'VALIDATION_ERROR','Reason must have at least 5 characters');
        c.status=decision==='RESOLVE'?'RESOLVED':decision==='REOPEN'?'OPEN':c.status;c.version+=1;addEvent(s,c,'DECISION_RECORDED',String(body.reason));}
      c.updated_at=iso();audit(s,seg[3]==='assign'?'CASE_ASSIGNED':'CASE_DECISION','CASE',c.id);return reply(200,c);
   }
 }
 if(method==='GET' && p==='/v1/audit-events')return output(200,paginate(s.audit,url));
 if(method==='GET' && p==='/v1/demo/scenarios'){
  const data:T.ScenarioList={items:(catalog.cases as Array<{key:T.ScenarioDefinition['key'];total_steps:number}>).map(c=>({key:c.key,title:c.key.split('_').map(w=>w[0].toUpperCase()+w.slice(1)).join(' '),description:({normal_flow:'A clean payment-to-settlement flow',delayed_settlement:'Gateway payment appears before the final settlement',duplicate_webhook:'Repeated notification without a second payment',provider_outage:'Bank source becomes temporarily unavailable'} as Record<string,string>)[c.key],steps:c.total_steps}))};
  return output(200,data);
 }
 if(method==='POST' && seg[1]==='demo' && seg[2]==='scenarios' && seg[4]==='runs'){
  const def=catalog.cases.find(c=>c.key===seg[3]);if(!def)return error(404,'NOT_FOUND','Unknown scenario');
  const run:T.ScenarioRun={id:uuid(),scenario_key:def.key as T.ScenarioRun['scenario_key'],status:'READY',step:0,total_steps:def.total_steps,virtual_now:iso(),organization_id:orgId,created_at:iso()};
  s.scenarioRuns.unshift(run);audit(s,'SCENARIO_STARTED','SCENARIO_RUN',run.id);return reply(201,run);
 }
 if(seg[1]==='demo' && seg[2]==='scenario-runs' && seg[3]){
  const run=s.scenarioRuns.find(x=>x.id===seg[3]);if(!run)return error(404,'NOT_FOUND','Scenario run not found');
  if(method==='GET' && seg.length===4)return output(200,run);
  if(method==='POST' && seg[4]==='advance'){
   if(run.step!==body.expected_step)return error(409,'STEP_CONFLICT','Scenario step changed; refresh the run');
   if(run.status==='COMPLETED')return error(409,'ALREADY_COMPLETED','Scenario already completed');
   advance(s,run);return reply(200,run);
  }
 }
 if(method==='GET' && p==='/v1/imports')return output(200,{items:[],next_cursor:null});
 if(method==='GET' && seg[1]==='imports' && seg[2])return error(404,'NOT_FOUND','No CSV import in mock mode');
 return error(501,'MOCK_NOT_IMPLEMENTED',`${method} ${p} is not implemented in the frontend mock; see OpenAPI contract.`);
}
