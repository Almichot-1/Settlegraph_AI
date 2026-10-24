#!/usr/bin/env python3
from pathlib import Path
import json
ROOT=Path(__file__).resolve().parents[1]
F=ROOT/'fixtures';F.mkdir(exist_ok=True)
ORG='11111111-1111-4111-8111-111111111111'
RUN='22222222-2222-4222-8222-222222222222'
USER='33333333-3333-4333-8333-333333333333'
CASE='44444444-4444-4444-8444-444444444444'
P='55555555-5555-4555-8555-555555555555'
ST='66666666-6666-4666-8666-666666666666'
EV1='77777777-7777-4777-8777-777777777777'
EV2='88888888-8888-4888-8888-888888888888'
T='2026-10-24T09:00:00Z'

def save(name,obj): (F/name).write_text(json.dumps(obj,indent=2,ensure_ascii=False)+'\n')

scenario_catalog={'catalog_version':'v1','currency':'ETB','seed_policy':'Fixed seeded test fixtures; virtual time starts 2026-10-24T09:00:00Z','cases':[
 {'key':'normal_flow','seed':101,'total_steps':2,'expected':{'unique_payment_observations':1,'unique_settlement_observations':1,'order_payment_matches':1,'payment_settlement_matches':1,'case_count':0,'ledger_balanced':True},'steps':[{'step':1,'action':'accept_2500_payment','event':'PAYMENT_ACCEPTED'},{'step':2,'action':'post_2500_settlement','event':'SETTLEMENT_POSTED'}]},
 {'key':'delayed_settlement','seed':102,'total_steps':3,'expected':{'unique_payment_observations':1,'unique_settlement_observations':1,'order_payment_matches':1,'payment_settlement_matches':1,'case_count_after_grace':1,'case_status_after_recovery':'EVIDENCE_UPDATED','ledger_balanced':True},'steps':[{'step':1,'action':'accept_2500_payment','event':'PAYMENT_ACCEPTED'},{'step':2,'action':'advance_virtual_clock_past_settlement_due','event':None,'expected_case':'SETTLEMENT_NOT_OBSERVED'},{'step':3,'action':'post_and_deliver_2500_settlement','event':'SETTLEMENT_POSTED','expected_case_status':'EVIDENCE_UPDATED'}]},
 {'key':'duplicate_webhook','seed':103,'total_steps':2,'expected':{'unique_payment_observations':1,'unique_settlement_observations':1,'duplicate_event_ack_count_min':1,'order_payment_matches':1,'payment_settlement_matches':1,'ledger_balanced':True},'steps':[{'step':1,'action':'accept_2500_payment_and_emit_identical_event_twice','event':'PAYMENT_ACCEPTED'},{'step':2,'action':'post_2500_settlement','event':'SETTLEMENT_POSTED'}]},
 {'key':'provider_outage','seed':104,'total_steps':3,'expected':{'unique_payment_observations':1,'unique_settlement_observations_after_backfill':1,'before_recovery_coverage':'INCOMPLETE_SOURCE','after_recovery_coverage':'COMPLETE','ledger_balanced':True},'steps':[{'step':1,'action':'accept_2500_payment','event':'PAYMENT_ACCEPTED'},{'step':2,'action':'post_settlement_but_hide_callback_and_disable_provider_api','event':None,'expected_coverage':'INCOMPLETE_SOURCE'},{'step':3,'action':'recover_api_and_backfill','event':'SETTLEMENT_POSTED','expected_coverage':'COMPLETE'}]}
]}
save('scenario-catalog.json',scenario_catalog)

payment={'event_id':EV1,'provider_id':'BANK_A','scenario_run_id':RUN,'sequence':1,'event_type':'PAYMENT_ACCEPTED','occurred_at':T,'transfer_reference':'SIM-TRF-1045','order_reference':'ORD-1045','payment_reference':'SIM-TRF-1045','amount':{'minor_units':'250000','currency':'ETB'},'payload_version':1}
settlement={**payment,'event_id':EV2,'provider_id':'BANK_B','sequence':1,'event_type':'SETTLEMENT_POSTED','occurred_at':'2026-10-24T11:00:00Z'}
save('bank-event-payment.json',payment)
save('bank-event-settlement.json',settlement)
case={'id':CASE,'kind':'SETTLEMENT_NOT_OBSERVED','status':'OPEN','severity':'MEDIUM','title':'Settlement not yet observed for SIM-TRF-1045','summary':'No settlement record in the covered source export after the configured grace period.','version':3,'created_at':T,'updated_at':'2026-10-24T11:00:00Z','evidence_count':2,'assigned_to':USER,'scenario_run_id':RUN}
save('case-open.json',case)
save('case-evidence.json',{'items':[{'id':P,'record_type':'PAYMENT','reference':'SIM-TRF-1045','amount':{'minor_units':'250000','currency':'ETB'},'occurred_at':T,'provider_id':'BANK_A','raw_payload_sha256':None},{'id':ST,'record_type':'SOURCE_STATUS','reference':'BANK_B:complete-through-11:00Z','amount':None,'occurred_at':'2026-10-24T11:00:00Z','provider_id':'BANK_B','raw_payload_sha256':None}],'next_cursor':None})
save('scenario-run.json',{'id':RUN,'scenario_key':'delayed_settlement','status':'RUNNING','step':2,'total_steps':3,'virtual_now':'2026-10-24T11:00:00Z','organization_id':ORG,'created_at':T})
save('dashboard-summary.json',{'as_of':'2026-10-24T11:00:00Z','orders_expected':{'minor_units':'250000','currency':'ETB'},'payments_accepted':{'minor_units':'250000','currency':'ETB'},'settlements_observed':{'minor_units':'0','currency':'ETB'},'unmatched_payments':{'minor_units':'250000','currency':'ETB'},'open_cases':1,'providers':[{'id':'BANK_A','name':'Simulated Bank A','kind':'BANK','sync_status':'HEALTHY','last_observed_at':'2026-10-24T11:00:00Z'},{'id':'BANK_B','name':'Simulated Bank B','kind':'BANK','sync_status':'HEALTHY','last_observed_at':'2026-10-24T11:00:00Z'}],'coverage':{'source_status':'COMPLETE','is_complete':True}})
save('ai-investigation-valid.json',{'summary':'The payment is accepted by simulated provider A, but no related settlement is observed in provider B through the reported source watermark.','hypotheses':[{'explanation':'Settlement posting may have been delayed in this seeded scenario.','evidence_ids':[P,ST]}],'recommended_actions':['Request settlement confirmation using transfer reference SIM-TRF-1045.'],'evidence_ids':[P,ST],'limitations':['Provider observations describe a synthetic test environment, not actual bank custody.']})
save('ai-investigation-invalid-fabricated-id.json',{'summary':'The transfer definitely settled.','hypotheses':[{'explanation':'I invented a bank entry.','evidence_ids':['99999999-9999-4999-8999-999999999999']}],'recommended_actions':['Mark paid now.'],'evidence_ids':['99999999-9999-4999-8999-999999999999'],'limitations':[]})
save('demo-session-response.json',{'access_token':'<redacted-demo-jwt>','token_type':'Bearer','expires_at':'2026-10-24T10:00:00Z','user':{'id':USER,'display_name':'Demo Owner','memberships':[{'organization':{'id':ORG,'name':'Sample Merchant','mode':'DEMO'},'role':'OWNER'}]},'default_organization_id':ORG})
print('wrote fixtures')
