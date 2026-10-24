#!/usr/bin/env python3
"""Generate the normative SettleGraph API and BankSim contracts from one typed catalog."""
from pathlib import Path
import yaml, json
ROOT=Path(__file__).resolve().parents[1]
C=ROOT/'contracts'; C.mkdir(exist_ok=True)

def S(typ, **kw): return {'type':typ, **kw}
def ref(name): return {'$ref':f'#/components/schemas/{name}'}
def fields(required, **props): return S('object', required=required, properties=props, additionalProperties=False)
def en(*xs): return S('string',enum=list(xs))
def string(**kw): return S('string',**kw)
def integer(**kw): return S('integer',**kw)
def uuid(): return string(format='uuid')
def dt(): return string(format='date-time')
def array(it): return S('array',items=it)
def nullable(s): return {**s, 'nullable':True}
def pg(item): return fields(['items','next_cursor'], items=array(ref(item)),next_cursor=nullable(string()))

money=fields(['minor_units','currency'],minor_units=string(pattern=r'^-?[0-9]+$'),currency=en('ETB'))
error=fields(['error'], error=fields(['code','message','request_id'],code=string(),message=string(),request_id=string(),details=S('object',additionalProperties=True)))
SC={
 'Money':money,
 'ApiError':error,
 'Role':en('OWNER','ANALYST','REVIEWER','VIEWER'),
 'Organization':fields(['id','name','mode'],id=uuid(),name=string(),mode=en('DEMO','PRODUCTION')),
 'Membership':fields(['organization','role'],organization=ref('Organization'),role=ref('Role')),
 'UserProfile':fields(['id','display_name','memberships'],id=uuid(),display_name=string(),memberships=array(ref('Membership'))),
 'DemoSessionRequest':fields(['display_name'],display_name=string(minLength=1,maxLength=80)),
 'DemoSessionResponse':fields(['access_token','token_type','expires_at','user','default_organization_id'],access_token=string(),token_type=en('Bearer'),expires_at=dt(),user=ref('UserProfile'),default_organization_id=uuid()),
 'HealthStatus':fields(['status','service'],status=en('UP','NOT_READY'),service=string()),
 'Provider':fields(['id','name','kind','sync_status','last_observed_at'],id=string(),name=string(),kind=en('BANK','WALLET','GATEWAY'),sync_status=en('HEALTHY','STALE','OUTAGE','NEVER_SYNCED'),last_observed_at=nullable(dt())),
 'ProviderList':pg('Provider'),
 'DashboardSummary':fields(['as_of','orders_expected','payments_accepted','settlements_observed','unmatched_payments','open_cases','providers','coverage'],as_of=dt(),orders_expected=ref('Money'),payments_accepted=ref('Money'),settlements_observed=ref('Money'),unmatched_payments=ref('Money'),open_cases=integer(minimum=0),providers=array(ref('Provider')),coverage=fields(['source_status','is_complete'],source_status=en('COMPLETE','INCOMPLETE_SOURCE','STALE'),is_complete=S('boolean'))),
 'TimePoint':fields(['bucket_start','payments_accepted','settlements_observed'],bucket_start=dt(),payments_accepted=ref('Money'),settlements_observed=ref('Money')),
 'DashboardSeries':fields(['granularity','points'],granularity=en('HOUR','DAY'),points=array(ref('TimePoint'))),
 'AccountView':fields(['id','provider_id','account_label','account_type','last_observed_at'],id=uuid(),provider_id=string(),account_label=string(),account_type=en('MERCHANT','CUSTOMER'),last_observed_at=nullable(dt()),observed_balance=ref('Money')),
 'AccountPage':pg('AccountView'),
 'Order':fields(['id','external_reference','amount','status','created_at'],id=uuid(),external_reference=string(),amount=ref('Money'),status=en('OPEN','PAYMENT_OBSERVED','RECONCILED'),created_at=dt(),scenario_run_id=nullable(uuid())),
 'OrderPage':pg('Order'),
 'Transaction':fields(['id','provider_id','external_reference','amount','state','occurred_at','observed_at','source_attestation'],id=uuid(),provider_id=string(),external_reference=string(),amount=ref('Money'),state=en('ACCEPTED','FAILED','REFUNDED'),occurred_at=dt(),observed_at=dt(),order_reference=nullable(string()),source_attestation=en('SIGNED_SIMULATED_PROVIDER','IMPORTED_UNVERIFIED'),scenario_run_id=nullable(uuid())),
 'TransactionPage':pg('Transaction'),
 'Settlement':fields(['id','provider_id','external_reference','payment_reference','amount','occurred_at','observed_at'],id=uuid(),provider_id=string(),external_reference=string(),payment_reference=string(),amount=ref('Money'),occurred_at=dt(),observed_at=dt(),scenario_run_id=nullable(uuid())),
 'SettlementPage':pg('Settlement'),
 'CreateRunRequest':fields(['scope'],scope=en('LATEST_OBSERVATIONS','SCENARIO_RUN'),scenario_run_id=nullable(uuid()),rules_version=string(default='v1')),
 'ReconciliationRun':fields(['id','status','scope','rules_version','created_at','matched_order_payment','matched_payment_settlement','unmatched_count'],id=uuid(),status=en('QUEUED','RUNNING','COMPLETED','FAILED'),scope=en('LATEST_OBSERVATIONS','SCENARIO_RUN'),rules_version=string(),created_at=dt(),completed_at=nullable(dt()),matched_order_payment=integer(minimum=0),matched_payment_settlement=integer(minimum=0),unmatched_count=integer(minimum=0),scenario_run_id=nullable(uuid())),
 'RunPage':pg('ReconciliationRun'),
 'Match':fields(['id','run_id','stage','left_evidence_id','right_evidence_id','allocated_amount','rule','proof_id'],id=uuid(),run_id=uuid(),stage=en('ORDER_PAYMENT','PAYMENT_SETTLEMENT'),left_evidence_id=uuid(),right_evidence_id=uuid(),allocated_amount=ref('Money'),rule=en('EXACT_PROVIDER_REFERENCE_AND_AMOUNT','EXACT_ORDER_REFERENCE_AND_AMOUNT'),proof_id=uuid()),
 'MatchPage':pg('Match'),
 'MatchProof':fields(['id','run_id','rule_version','stage','checks','source_evidence_ids','result'],id=uuid(),run_id=uuid(),rule_version=string(),stage=en('ORDER_PAYMENT','PAYMENT_SETTLEMENT'),checks=array(string()),source_evidence_ids=array(uuid()),result=en('MATCHED_IN_IMPORTED_RECORDS')),
 'Evidence':fields(['id','record_type','reference','amount','occurred_at','provider_id'],id=uuid(),record_type=en('ORDER','PAYMENT','SETTLEMENT','SOURCE_STATUS'),reference=string(),amount=nullable(ref('Money')),occurred_at=dt(),provider_id=nullable(string()),raw_payload_sha256=nullable(string())),
 'EvidencePage':pg('Evidence'),
 'Case':fields(['id','kind','status','severity','title','summary','version','created_at','updated_at','evidence_count'],id=uuid(),kind=en('SETTLEMENT_NOT_OBSERVED','AMOUNT_MISMATCH','AMBIGUOUS_MATCH','SOURCE_UNAVAILABLE'),status=en('OPEN','EVIDENCE_UPDATED','RESOLVED'),severity=en('LOW','MEDIUM','HIGH'),title=string(),summary=string(),version=integer(minimum=1),created_at=dt(),updated_at=dt(),evidence_count=integer(minimum=0),assigned_to=nullable(uuid()),scenario_run_id=nullable(uuid())),
 'CasePage':pg('Case'),
 'CaseEvent':fields(['id','case_id','type','occurred_at','actor','message','version'],id=uuid(),case_id=uuid(),type=en('OPENED','EVIDENCE_ADDED','INVESTIGATION_REQUESTED','INVESTIGATION_COMPLETED','ASSIGNED','DECISION_RECORDED','RESOLVED'),occurred_at=dt(),actor=string(),message=string(),version=integer(minimum=1)),
 'CaseEventPage':pg('CaseEvent'),
 'AssignCaseRequest':fields(['assignee_id'],assignee_id=uuid()),
 'ReviewDecisionRequest':fields(['decision','reason'],decision=en('RESOLVE','REOPEN','NEEDS_MORE_EVIDENCE'),reason=string(minLength=5,maxLength=2000)),
 'InvestigationRequest':fields(['mode'],mode=en('EVIDENCE_ANALYSIS')),
 'Investigation':fields(['id','case_id','status','provider','created_at','evidence_snapshot_hash'],id=uuid(),case_id=uuid(),status=en('QUEUED','RUNNING','COMPLETED','REJECTED','FAILED'),provider=en('BEDROCK','OFFLINE_STUB'),created_at=dt(),evidence_snapshot_hash=string(),completed_at=nullable(dt()),result=ref('InvestigationResult')),
 'InvestigationResult':fields(['summary','hypotheses','recommended_actions','evidence_ids','limitations'],summary=string(),hypotheses=array(fields(['explanation','evidence_ids'],explanation=string(),evidence_ids=array(uuid()))),recommended_actions=array(string()),evidence_ids=array(uuid()),limitations=array(string())),
 'InvestigationPage':pg('Investigation'),
 'Job':fields(['id','kind','status','created_at'],id=uuid(),kind=en('RECONCILE','INVESTIGATE','IMPORT','BACKFILL'),status=en('QUEUED','RUNNING','COMPLETED','FAILED'),created_at=dt(),finished_at=nullable(dt()),error_code=nullable(string())),
 'AuditEvent':fields(['id','actor','action','resource_type','resource_id','occurred_at'],id=uuid(),actor=string(),action=string(),resource_type=string(),resource_id=string(),occurred_at=dt()),
 'AuditEventPage':pg('AuditEvent'),
 'ScenarioDefinition':fields(['key','title','description','steps'],key=en('normal_flow','delayed_settlement','duplicate_webhook','provider_outage'),title=string(),description=string(),steps=integer(minimum=1)),
 'ScenarioList':fields(['items'],items=array(ref('ScenarioDefinition'))),
 'StartScenarioRequest':fields(['seed'],seed=integer(minimum=0,maximum=2147483647)),
 'ScenarioRun':fields(['id','scenario_key','status','step','total_steps','virtual_now','organization_id','created_at'],id=uuid(),scenario_key=en('normal_flow','delayed_settlement','duplicate_webhook','provider_outage'),status=en('PREPARING','READY','RUNNING','COMPLETED','FAILED'),step=integer(minimum=0),total_steps=integer(minimum=1),virtual_now=dt(),organization_id=uuid(),created_at=dt()),
 'AdvanceScenarioRequest':fields(['expected_step'],expected_step=integer(minimum=0)),
 'WebhookAccepted':fields(['accepted','duplicate'],accepted=S('boolean'),duplicate=S('boolean')),
 'Import':fields(['id','status','filename','source_kind','created_at','accepted_records','rejected_records'],id=uuid(),status=en('AWAITING_UPLOAD','QUEUED','PROCESSING','COMPLETED','FAILED'),filename=string(),source_kind=en('ORDERS','PAYMENTS','SETTLEMENTS'),created_at=dt(),accepted_records=integer(minimum=0),rejected_records=integer(minimum=0)),
 'ImportPage':pg('Import'),
 'BankEvent':fields(['event_id','provider_id','scenario_run_id','sequence','event_type','occurred_at','transfer_reference','amount','payload_version'],event_id=uuid(),provider_id=string(),scenario_run_id=uuid(),sequence=integer(minimum=1),event_type=en('PAYMENT_ACCEPTED','SETTLEMENT_POSTED','REFUND_POSTED','TRANSFER_FAILED'),occurred_at=dt(),transfer_reference=string(),amount=ref('Money'),payload_version=integer(enum=[1]),order_reference=nullable(string()),payment_reference=nullable(string())),
 'CreateImportRequest':fields(['filename','source_kind','content_sha256','file_size_bytes'],filename=string(maxLength=160),source_kind=en('ORDERS','PAYMENTS','SETTLEMENTS'),content_sha256=string(pattern='^[a-f0-9]{64}$'),file_size_bytes=integer(minimum=1,maximum=5242880)),
 'ImportUploadSession':fields(['import','upload_url','upload_headers','expires_at'],**{'import':ref('Import'),'upload_url':string(format='uri'),'upload_headers':S('object',additionalProperties=string()),'expires_at':dt()}),
 'CompleteImportRequest':fields(['content_sha256'],content_sha256=string(pattern='^[a-f0-9]{64}$')),
}

p={
 'Org':{'name':'X-Organization-Id','in':'header','required':True,'schema':uuid(),'description':'Organization must belong to bearer subject; server verifies membership.'},
 'Idem':{'name':'Idempotency-Key','in':'header','required':True,'schema':string(minLength=8,maxLength=128),'description':'Scoped to organization, actor, route, and request hash; mismatch => 409.'},
 'IfMatch':{'name':'If-Match','in':'header','required':True,'schema':string(pattern='^"[0-9]+"$'),'description':'Quoted integer from Case.version; stale => 412; required => 428.'},
 'Limit':{'name':'limit','in':'query','required':False,'schema':integer(minimum=1,maximum=100,default=20)},
 'Cursor':{'name':'cursor','in':'query','required':False,'schema':string()},
 'CaseId':{'name':'caseId','in':'path','required':True,'schema':uuid()},
 'RunId':{'name':'runId','in':'path','required':True,'schema':uuid()},
 'ScenarioKey':{'name':'scenarioKey','in':'path','required':True,'schema':en('normal_flow','delayed_settlement','duplicate_webhook','provider_outage')},
 'ScenarioRunId':{'name':'scenarioRunId','in':'path','required':True,'schema':uuid()},
 'InvestigationId':{'name':'investigationId','in':'path','required':True,'schema':uuid()},
 'Id':{'name':'id','in':'path','required':True,'schema':uuid()},
}
RESP={}
for code,desc in [('BadRequest','Invalid request'),('Unauthorized','Missing or invalid authentication'),('Forbidden','Permission denied'),('NotFound','Resource not found'),('Conflict','Conflicting request'),('PreconditionFailed','Stale version or step'),('PreconditionRequired','Missing If-Match'),('RateLimited','Rate limited'),('ServiceUnavailable','Temporarily unavailable')]:
 RESP[code]={'description':desc,'content':{'application/json':{'schema':ref('ApiError')}}}

OPEN={'openapi':'3.0.3', 'info':{'title':'SettleGraph AI — Web + Mobile Public API','version':'2.0.0-draft','description':'Normative proposed client contract, not yet implemented. All financial amounts are decimal-string minor units. Simulated provider data is not authenticated real-world bank balance information.'},
 'servers':[{'url':'http://localhost:8080','description':'Local development; HTTPS in deployment'}],
 'security':[{'BearerAuth':[]}],
 'tags':[{'name':x} for x in ['Health','Identity','Providers','Dashboard','Orders','Transactions','Settlements','Reconciliation','Cases','Investigations','Audit','Demo','Ingestion','Imports']],
 'paths':{},'components':{'securitySchemes':{'BearerAuth':{'type':'http','scheme':'bearer','bearerFormat':'JWT'},'DemoCode':{'type':'apiKey','in':'header','name':'X-Demo-Access-Code'},'WebhookSignature':{'type':'apiKey','in':'header','name':'X-BankSim-Signature'}},'schemas':SC,'parameters':p,'responses':RESP}}

def P(k): return {'$ref':f'#/components/parameters/{k}'}
def RE(name): return {'$ref':f'#/components/responses/{name}'}
def response(model,desc='OK',status='200'):
 return {status:{'description':desc,'content':{'application/json':{'schema':ref(model)}}}}
def body(model): return {'required':True,'content':{'application/json':{'schema':ref(model)}}}
def operation(method,path,tag,opid,summary,model=None,success='200',params=(),request=None,security=None,errors=('BadRequest','Unauthorized','Forbidden','NotFound','Conflict'),description=None,extra=None):
 o={'tags':[tag], 'operationId':opid,'summary':summary}
 if description:o['description']=description
 if params:o['parameters']=[P(v) for v in params]
 if request:o['requestBody']=body(request)
 if security is not None:o['security']=security
 o['responses']=(response(model,status=success,desc='Success') if model else {success:{'description':'Success'}})
 for e in errors:
  code={'BadRequest':'400','Unauthorized':'401','Forbidden':'403','NotFound':'404','Conflict':'409','PreconditionFailed':'412','PreconditionRequired':'428','RateLimited':'429','ServiceUnavailable':'503'}[e]
  if code not in o['responses']:o['responses'][code]=RE(e)
 if extra:o.update(extra)
 OPEN['paths'].setdefault(path,{})[method]=o

operation('get','/health/live','Health','getLiveness','Process liveness','HealthStatus',security=[],errors=())
operation('get','/health/ready','Health','getReadiness','Required dependency readiness','HealthStatus',security=[],errors=('ServiceUnavailable',))
operation('post','/v1/demo/sessions','Identity','createDemoSession','Mint demo-only session token','DemoSessionResponse','201',request='DemoSessionRequest',security=[{'DemoCode':[]}],errors=('BadRequest','RateLimited','NotFound'),description='Only enabled when APP_MODE=demo. Never expose demo minting in production.')
operation('get','/v1/me','Identity','getMe','Current user and organization memberships','UserProfile',errors=('Unauthorized',))
operation('get','/v1/providers','Providers','listProviders','Provider sync and availability status','ProviderList',params=('Org','Limit','Cursor'))
operation('get','/v1/accounts','Providers','listAccounts','Known business account views','AccountPage',params=('Org','Limit','Cursor'))
operation('get','/v1/dashboard/summary','Dashboard','getDashboardSummary','Payment and settlement summary from observations','DashboardSummary',params=('Org',),description='Different stages are reported separately; never sum orders, payments and settlements as one monetary total.')
operation('get','/v1/dashboard/series','Dashboard','getDashboardSeries','Time bucket series','DashboardSeries',params=('Org',), extra={'parameters':[P('Org'),{'name':'granularity','in':'query','required':False,'schema':en('HOUR','DAY')},{'name':'from','in':'query','required':True,'schema':dt()},{'name':'to','in':'query','required':True,'schema':dt()}]})
operation('get','/v1/orders','Orders','listOrders','List known merchant orders','OrderPage',params=('Org','Limit','Cursor'))
operation('get','/v1/orders/{id}','Orders','getOrder','Get merchant order','Order',params=('Org','Id'))
operation('get','/v1/transactions','Transactions','listTransactions','List normalized accepted/failed/refunded payment observations','TransactionPage',params=('Org','Limit','Cursor'),extra={'parameters':[P('Org'),P('Limit'),P('Cursor'),{'name':'provider_id','in':'query','schema':string()},{'name':'scenario_run_id','in':'query','schema':uuid()}]})
operation('get','/v1/transactions/{id}','Transactions','getTransaction','Get single payment observation','Transaction',params=('Org','Id'))
operation('get','/v1/settlements','Settlements','listSettlements','List observed settlement records','SettlementPage',params=('Org','Limit','Cursor'))
operation('get','/v1/settlements/{id}','Settlements','getSettlement','Get settlement observation','Settlement',params=('Org','Id'))
operation('get','/v1/reconciliation/runs','Reconciliation','listReconciliationRuns','List reconciliation attempts','RunPage',params=('Org','Limit','Cursor'))
operation('post','/v1/reconciliation/runs','Reconciliation','createReconciliationRun','Queue a new deterministic reconciliation run','ReconciliationRun','202',params=('Org','Idem'),request='CreateRunRequest',errors=('BadRequest','Unauthorized','Forbidden','Conflict','ServiceUnavailable'))
operation('get','/v1/reconciliation/runs/{runId}','Reconciliation','getReconciliationRun','Get reconciliation status and counts','ReconciliationRun',params=('Org','RunId'))
operation('get','/v1/reconciliation/runs/{runId}/matches','Reconciliation','listRunMatches','Get evidence-backed matched edges','MatchPage',params=('Org','RunId','Limit','Cursor'))
operation('get','/v1/reconciliation/proofs/{id}','Reconciliation','getMatchProof','Get checkable match proof','MatchProof',params=('Org','Id'))
operation('get','/v1/cases','Cases','listCases','List cases (including mobile assigned queue)','CasePage',params=('Org','Limit','Cursor'),extra={'parameters':[P('Org'),P('Limit'),P('Cursor'),{'name':'status','in':'query','schema':en('OPEN','EVIDENCE_UPDATED','RESOLVED')},{'name':'assigned_to','in':'query','schema':uuid()},{'name':'scenario_run_id','in':'query','schema':uuid()}]})
operation('get','/v1/cases/{caseId}','Cases','getCase','Get case summary with version','Case',params=('Org','CaseId'))
operation('get','/v1/cases/{caseId}/evidence','Cases','listCaseEvidence','Evidence available at latest case version','EvidencePage',params=('Org','CaseId','Limit','Cursor'))
operation('get','/v1/cases/{caseId}/events','Cases','listCaseEvents','Immutable case timeline','CaseEventPage',params=('Org','CaseId','Limit','Cursor'))
operation('post','/v1/cases/{caseId}/assign','Cases','assignCase','Assign reviewer (role OWNER or ANALYST)','Case','200',params=('Org','CaseId','Idem','IfMatch'),request='AssignCaseRequest',errors=('BadRequest','Unauthorized','Forbidden','NotFound','Conflict','PreconditionRequired','PreconditionFailed'))
operation('post','/v1/cases/{caseId}/decisions','Cases','recordCaseDecision','Record reviewer decision; financial truth unchanged','Case','200',params=('Org','CaseId','Idem','IfMatch'),request='ReviewDecisionRequest',errors=('BadRequest','Unauthorized','Forbidden','NotFound','Conflict','PreconditionRequired','PreconditionFailed'))
operation('post','/v1/cases/{caseId}/investigations','Investigations','createInvestigation','Queue read-only AI analysis for current evidence snapshot','Investigation','202',params=('Org','CaseId','Idem'),request='InvestigationRequest',errors=('BadRequest','Unauthorized','Forbidden','NotFound','Conflict','ServiceUnavailable'))
operation('get','/v1/cases/{caseId}/investigations','Investigations','listInvestigations','View AI investigation history','InvestigationPage',params=('Org','CaseId','Limit','Cursor'))
operation('get','/v1/cases/{caseId}/investigations/{investigationId}','Investigations','getInvestigation','Retrieve evidence-gated AI result','Investigation',params=('Org','CaseId','InvestigationId'))
operation('get','/v1/jobs/{id}','Investigations','getJob','Get async job status','Job',params=('Org','Id'))
operation('get','/v1/audit-events','Audit','listAuditEvents','Read immutable audit trail','AuditEventPage',params=('Org','Limit','Cursor'))
operation('get','/v1/demo/scenarios','Demo','listDemoScenarios','List permitted simulator demonstrations','ScenarioList',params=('Org',),errors=('Unauthorized','Forbidden','NotFound'))
operation('post','/v1/demo/scenarios/{scenarioKey}/runs','Demo','createScenarioRun','Create fresh isolated seeded simulation run','ScenarioRun','201',params=('Org','ScenarioKey','Idem'),request='StartScenarioRequest',errors=('BadRequest','Unauthorized','Forbidden','NotFound','Conflict','ServiceUnavailable'))
operation('get','/v1/demo/scenario-runs/{scenarioRunId}','Demo','getScenarioRun','Get scenario progress and virtual clock','ScenarioRun',params=('Org','ScenarioRunId'))
operation('post','/v1/demo/scenario-runs/{scenarioRunId}/advance','Demo','advanceScenarioRun','Advance a scenario by one deterministic step','ScenarioRun','200',params=('Org','ScenarioRunId','Idem'),request='AdvanceScenarioRequest',errors=('BadRequest','Unauthorized','Forbidden','NotFound','Conflict','PreconditionFailed','ServiceUnavailable'))
operation('post','/v1/integrations/banksim/events','Ingestion','acceptBankSimEvent','Ingest signed BankSim webhook','WebhookAccepted','202',security=[{'WebhookSignature':[]}],errors=('BadRequest','Unauthorized','ServiceUnavailable'),description='Requires X-BankSim-Event-Id, X-BankSim-Delivery-Timestamp (epoch seconds), and X-BankSim-Signature: v1=<lowercase hex HMAC SHA256>. Signature input timestamp + "." + event_id + "." + exact raw body. Delivery freshness ±300 seconds.',extra={'parameters':[{'name':'X-BankSim-Event-Id','in':'header','required':True,'schema':uuid()},{'name':'X-BankSim-Delivery-Timestamp','in':'header','required':True,'schema':string(pattern='^[0-9]+$')},{'name':'X-BankSim-Signature','in':'header','required':True,'schema':string(pattern='^v1=[a-f0-9]{64}$')}],'requestBody':{'required':True,'content':{'application/json':{'schema':ref('BankEvent')}}}})
# Optional fallback import endpoints, deliberately listed at end.
operation('get','/v1/imports','Imports','listImports','[Fallback] List uploaded evidence','ImportPage',params=('Org','Limit','Cursor'))
operation('post','/v1/imports','Imports','createImport','[Fallback] Create S3 presigned import','ImportUploadSession','201',params=('Org','Idem'),request='CreateImportRequest')
operation('get','/v1/imports/{id}','Imports','getImport','[Fallback] Get ingestion progress','Import',params=('Org','Id'))
operation('post','/v1/imports/{id}/complete','Imports','completeImport','[Fallback] Verify uploaded blob and queue parser','Import','202',params=('Org','Id','Idem'),request='CompleteImportRequest')

P0=set('getLiveness getReadiness createDemoSession getMe listProviders getDashboardSummary listTransactions getTransaction listSettlements createReconciliationRun getReconciliationRun listRunMatches listCases getCase listCaseEvidence listCaseEvents recordCaseDecision createInvestigation listInvestigations listDemoScenarios createScenarioRun getScenarioRun advanceScenarioRun acceptBankSimEvent'.split())
for route, methods in OPEN['paths'].items():
 for verb,o in methods.items():
  o['x-implementation-priority']=('P0' if o['operationId'] in P0 else ('P2' if o['tags'][0]=='Imports' else 'P1'))
  o['x-client']=('service-only' if o['tags'][0]=='Ingestion' else ('web-flutter' if o['tags'][0] in ('Identity','Cases','Investigations') else 'web-primary'))

with (C/'settlegraph-public.openapi.yaml').open('w') as f:yaml.safe_dump(OPEN,f,sort_keys=False,allow_unicode=True,width=105)

# BankSim has independent token and database; no bearer end-user auth.
bsS={
 'Money':money,
 'SimInstitution':fields(['id','name','kind'],id=string(),name=string(),kind=en('BANK','WALLET')),
 'SimInstitutionList':fields(['items'],items=array(ref('SimInstitution'))),
 'SimAccount':fields(['id','institution_id','label','currency'],id=uuid(),institution_id=string(),label=string(),currency=en('ETB')),
 'SimTransferRequest':fields(['from_account_id','to_account_id','amount','order_reference'],from_account_id=uuid(),to_account_id=uuid(),amount=ref('Money'),order_reference=string()),
 'SimTransfer':fields(['id','from_account_id','to_account_id','amount','state','reference','created_at'],id=uuid(),from_account_id=uuid(),to_account_id=uuid(),amount=ref('Money'),state=en('CREATED','ACCEPTED','SETTLEMENT_PENDING','SETTLED','FAILED'),reference=string(),created_at=dt()),
 'ProviderEvent':fields(['event_id','provider_id','sequence','type','transfer_reference','occurred_at','amount','scenario_run_id'],event_id=uuid(),provider_id=string(),sequence=integer(minimum=1),type=en('PAYMENT_ACCEPTED','SETTLEMENT_POSTED','REFUND_POSTED','TRANSFER_FAILED'),transfer_reference=string(),occurred_at=dt(),amount=ref('Money'),scenario_run_id=uuid(),order_reference=nullable(string()),payment_reference=nullable(string())),
 'EventPage':fields(['items','next_cursor','source_status','complete_through'],items=array(ref('ProviderEvent')),next_cursor=nullable(string()),source_status=en('HEALTHY','OUTAGE','STALE'),complete_through=nullable(dt())),
 'ScenarioRun':SC['ScenarioRun'],
 'StartBankScenarioRequest':fields(['seed','organization_id'],seed=integer(minimum=0,maximum=2147483647),organization_id=uuid()),
 'AdvanceScenarioRequest':SC['AdvanceScenarioRequest'],
 'ScenarioList':SC['ScenarioList'],
 'ScenarioDefinition':SC['ScenarioDefinition'],
 'EvaluationTruth':fields(['scenario_run_id','seed','expected_payments','expected_settlements','source_ledger_balanced'],scenario_run_id=uuid(),seed=integer(),expected_payments=integer(),expected_settlements=integer(),source_ledger_balanced=S('boolean')),
 'ApiError':error,
 'HealthStatus':SC['HealthStatus'],
}
B={'openapi':'3.0.3','info':{'title':'BankSim — Independent Internal Simulator API','version':'2.0.0-draft','description':'Synthetic funds only. Provider read token, demo admin token and evaluator token are separate. SettleGraph MUST NOT access evaluator truth or simulator DB.'},'servers':[{'url':'http://localhost:8082'}],
 'security':[{'ProviderToken':[]}],
 'tags':[{'name':x} for x in ['Simulator','Provider','ScenarioAdmin','Evaluator']],
 'paths':{},'components':{'securitySchemes':{'ProviderToken':{'type':'http','scheme':'bearer'},'AdminToken':{'type':'http','scheme':'bearer'},'EvaluatorToken':{'type':'http','scheme':'bearer'}},'schemas':bsS,'parameters':{'InstitutionId':{'name':'institutionId','in':'path','required':True,'schema':string()},'RunId':p['RunId'],'ScenarioKey':p['ScenarioKey'],'Cursor':p['Cursor'],'Limit':p['Limit'],'Id':p['Id'],'Idem':p['Idem']},'responses':{'ApiError':{'description':'Error','content':{'application/json':{'schema':ref('ApiError')}}}}}}

def bop(method,path,tag,opid,summary,out,params=(),request=None,security=None,code='200',desc=''):
 o={'tags':[tag], 'operationId':opid,'summary':summary, 'responses':{code:{'description':'Success','content':{'application/json':{'schema':ref(out)}}},'400':{'$ref':'#/components/responses/ApiError'},'401':{'$ref':'#/components/responses/ApiError'},'403':{'$ref':'#/components/responses/ApiError'},'404':{'$ref':'#/components/responses/ApiError'},'409':{'$ref':'#/components/responses/ApiError'},'412':{'$ref':'#/components/responses/ApiError'}}}
 if params:o['parameters']=[{'$ref':f'#/components/parameters/{x}'} for x in params]
 if request:o['requestBody']={'required':True,'content':{'application/json':{'schema':ref(request)}}}
 if security is not None:o['security']=security
 if desc:o['description']=desc
 B['paths'].setdefault(path,{})[method]=o

bop('get','/health/live','Simulator','getBankSimHealth','Simulator liveness','HealthStatus',security=[])
bop('get','/provider/v1/institutions','Provider','listSimInstitutions','List institutions','SimInstitutionList')
bop('get','/provider/v1/institutions/{institutionId}/events','Provider','listBankEvents','Read source events for durable backfill','EventPage',params=('InstitutionId','Limit','Cursor'))
bop('post','/provider/v1/transfers','Provider','createSimTransfer','Create synthetic payment (admin demo only)','SimTransfer',params=('Idem',),request='SimTransferRequest',security=[{'AdminToken':[]}],code='201')
bop('get','/provider/v1/transfers/{id}','Provider','getSimTransfer','Get one synthetic transfer','SimTransfer',params=('Id',))
bop('get','/admin/v1/scenarios','ScenarioAdmin','listBankScenarios','List deterministic scenarios','ScenarioList',security=[{'AdminToken':[]}])
bop('post','/admin/v1/scenarios/{scenarioKey}/runs','ScenarioAdmin','createBankScenario','Start a new scenario','ScenarioRun',params=('ScenarioKey','Idem'),request='StartBankScenarioRequest',security=[{'AdminToken':[]}],code='201')
bop('get','/admin/v1/runs/{runId}','ScenarioAdmin','getBankScenario','Scenario state','ScenarioRun',params=('RunId',),security=[{'AdminToken':[]}])
bop('post','/admin/v1/runs/{runId}/advance','ScenarioAdmin','advanceBankScenario','Advance exactly one scenario step','ScenarioRun',params=('RunId','Idem'),request='AdvanceScenarioRequest',security=[{'AdminToken':[]}])
bop('get','/evaluator/v1/runs/{runId}/truth','Evaluator','getSimulationTruth','Test-only simulator ground truth','EvaluationTruth',params=('RunId',),security=[{'EvaluatorToken':[]}],desc='Evaluator-only credentials; MUST be denied to SettleGraph product processes.')
with (C/'banksim-internal.openapi.yaml').open('w') as f:yaml.safe_dump(B,f,sort_keys=False,allow_unicode=True,width=105)

# Bedrock-compatible schema: only basic types/required/additionalProperties (validate separately in Go).
ai={'$schema':'https://json-schema.org/draft/2020-12/schema','title':'SettleGraph evidence-grounded AI investigation','type':'object','additionalProperties':False,'required':['summary','hypotheses','recommended_actions','evidence_ids','limitations'], 'properties':{
 'summary':{'type':'string'},
 'hypotheses':{'type':'array','items':{'type':'object','additionalProperties':False,'required':['explanation','evidence_ids'],'properties':{'explanation':{'type':'string'},'evidence_ids':{'type':'array','items':{'type':'string'}}}}},
 'recommended_actions':{'type':'array','items':{'type':'string'}},
 'evidence_ids':{'type':'array','items':{'type':'string'}},
 'limitations':{'type':'array','items':{'type':'string'}}}}
(C/'ai-investigation.schema.json').write_text(json.dumps(ai,indent=2)+'\n')

event={'$schema':'https://json-schema.org/draft/2020-12/schema','title':'Simulated institution provider event','type':'object','additionalProperties':False,
 'required':['event_id','provider_id','scenario_run_id','sequence','event_type','occurred_at','transfer_reference','amount','payload_version'],
 'properties':{'event_id':{'type':'string','format':'uuid'},'provider_id':{'type':'string','minLength':1},'scenario_run_id':{'type':'string','format':'uuid'},'sequence':{'type':'integer','minimum':1},'event_type':{'enum':['PAYMENT_ACCEPTED','SETTLEMENT_POSTED','REFUND_POSTED','TRANSFER_FAILED']},'occurred_at':{'type':'string','format':'date-time'},'transfer_reference':{'type':'string','minLength':1},'order_reference':{'type':['string','null']},'payment_reference':{'type':['string','null']},'amount':{'type':'object','additionalProperties':False,'required':['minor_units','currency'],'properties':{'minor_units':{'type':'string','pattern':'^[0-9]+$'},'currency':{'const':'ETB'}}},'payload_version':{'const':1}}}
(C/'bank-event.schema.json').write_text(json.dumps(event,indent=2)+'\n')

# Explicit contract inventory for status tracking by all clients.
allops=[]
for path,items in OPEN['paths'].items():
 for method,o in items.items():allops.append({'method':method.upper(),'path':path,'operationId':o['operationId'],'tag':o['tags'][0],'success':next(iter(o['responses']))})
(ROOT/'API_ENDPOINT_INVENTORY.json').write_text(json.dumps(allops,indent=2)+'\n')
print(f'Generated {len(allops)} public endpoints and {sum(len(v) for v in B["paths"].values())} BankSim endpoints')
