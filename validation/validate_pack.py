#!/usr/bin/env python3
"""Offline contract checks. These are *static tests*, not running service or DB tests."""
from pathlib import Path
from collections import Counter
import json, sys, yaml
from jsonschema import Draft202012Validator, Draft4Validator, FormatChecker
ROOT=Path(__file__).resolve().parents[1]
C=ROOT/'contracts'; F=ROOT/'fixtures'
fail=[]

def check(cond,msg):
 if not cond: fail.append(msg)

def walk(obj,path=()):
 if isinstance(obj,dict):
  yield obj,path
  for k,v in obj.items(): yield from walk(v,path+(str(k),))
 elif isinstance(obj,list):
  for n,v in enumerate(obj): yield from walk(v,path+(str(n),))

for filename, expected_min in [('settlegraph-public.openapi.yaml',35),('banksim-internal.openapi.yaml',10)]:
 doc=yaml.safe_load((C/filename).read_text())
 check(doc.get('openapi')=='3.0.3',f'{filename}: OpenAPI version incorrect')
 paths=doc.get('paths',{})
 operations=[(method,path,op) for path,items in paths.items() for method,op in items.items() if method in ['get','post','put','delete','patch']]
 check(len(operations)>=expected_min,f'{filename}: too few operations: {len(operations)}')
 opids=[op.get('operationId') for _,_,op in operations]
 check(len(set(opids))==len(opids),f'{filename}: duplicate operationId')
 check(len(doc['components']['schemas'])>0,f'{filename}: no schema')
 for method,path,op in operations:
  check('responses' in op and len(op['responses'])>0,f'{filename} {method} {path}: responses missing')
  check(bool(op.get('tags')),f'{filename} {path}: tag missing')
  parameters=(doc['paths'][path].get('parameters',[])+op.get('parameters',[]))
  paramnames=set()
  for param in parameters:
   if '$ref' in param:
    try: param=doc['components']['parameters'][param['$ref'].split('/')[-1]]
    except KeyError:fail.append(f'{filename} {path}: unresolved param')
   if isinstance(param,dict):paramnames.add(param.get('name'))
  import re
  for name in re.findall(r'\{([^}]+)\}',path):
   check(name in paramnames,f'{filename}: missing path param {name} in {path}')
 for node,path in walk(doc):
  if '$ref' in node:
   r=node['$ref']
   if isinstance(r,str) and r.startswith('#/'):
    cur=doc
    try:
     for seg in r[2:].split('/'):cur=cur[seg]
    except (KeyError,IndexError,TypeError): fail.append(f'{filename}: broken ref {r} at {path}')
   else:fail.append(f'{filename}: unsupported external ref {r} at {path}')
 for name,schema in doc['components']['schemas'].items():
  check(schema.get('type') in (None,'object','array','string','integer','boolean'),f'{filename} schema {name}: invalid root type')
  for node,path in walk(schema):
   check(not isinstance(node.get('type'),list),f'{filename} schema {name}: OpenAPI 3.0 cannot use union type at {path}')
 print(f'{filename}: {len(operations)} operations, {len(doc["components"]["schemas"])} schemas, refs traversed')

for name in ['bank-event.schema.json','ai-investigation.schema.json']:
 schema=json.loads((C/name).read_text());Draft202012Validator.check_schema(schema)
 print(name, 'JSON Schema validates')

es=json.loads((C/'bank-event.schema.json').read_text())
for name in ['bank-event-payment.json','bank-event-settlement.json']:
 sample=json.loads((F/name).read_text())
 issues=list(Draft202012Validator(es,format_checker=FormatChecker()).iter_errors(sample))
 check(not issues,f'{name}: {issues[0] if issues else "OK"}')

ai=json.loads((C/'ai-investigation.schema.json').read_text())
valid=json.loads((F/'ai-investigation-valid.json').read_text())
check(not list(Draft202012Validator(ai).iter_errors(valid)),'AI valid fixture rejected')

# Verify AI semantic gate independently from syntactic validity.
wrong=json.loads((F/'ai-investigation-invalid-fabricated-id.json').read_text())
allowed={x['id'] for x in json.loads((F/'case-evidence.json').read_text())['items']}
check(any(i not in allowed for i in wrong['evidence_ids']), 'Adversarial fixture does not contain non-snapshot evidence')
check(all(i in allowed for i in valid['evidence_ids']),'Valid AI fixture has outside evidence IDs')

public=yaml.safe_load((C/'settlegraph-public.openapi.yaml').read_text())
schemas=public['components']['schemas']
# Convert OpenAPI v3.0 nullable fields into JSON Schema Draft 4 definitions for sample tests.
def strip_nullable(obj):
 if isinstance(obj,list):return [strip_nullable(x) for x in obj]
 if isinstance(obj,dict):
  out={k:strip_nullable(v) for k,v in obj.items() if k not in ('example','deprecated','readOnly','writeOnly')}
  if '$ref' in out and isinstance(out['$ref'],str):out['$ref']=out['$ref'].replace('#/components/schemas/','#/definitions/')
  if out.pop('nullable',False):
   # JSON Schema Draft 4 permits anyOf with null branch
   out={'anyOf':[out,{'type':'null'}]}
  if 'enum' in out and 'type' in out and out['type']=='integer':pass
  return out
 return obj
converted={k:strip_nullable(v) for k,v in schemas.items()}
for fixture,schema_name in [('case-open.json','Case'),('dashboard-summary.json','DashboardSummary'),('scenario-run.json','ScenarioRun'),('demo-session-response.json','DemoSessionResponse'),('case-evidence.json','EvidencePage')]:
 sch={'$ref':f'#/definitions/{schema_name}','definitions':converted}
 obj=json.loads((F/fixture).read_text())
 issues=list(Draft4Validator(sch,format_checker=FormatChecker()).iter_errors(obj))
 check(not issues,f'OpenAPI sample {fixture} vs {schema_name}: {issues[0] if issues else "OK"}')

catalog=json.loads((F/'scenario-catalog.json').read_text())
keys={x['key'] for x in catalog['cases']}
check(keys=={'normal_flow','delayed_settlement','duplicate_webhook','provider_outage'},'Scenario catalog misses required cases')
check(len([x for x in catalog['cases'] if x['expected']['ledger_balanced']])==4,'Fixture missing ledger balance expectation')

# Project-manager work order graph sanity checks (not proof of implementation).
backlog=json.loads((ROOT/'AGENT_BACKLOG.json').read_text())
tickets=backlog['tickets']; taskids=[t['id'] for t in tickets]
check(len(tickets)==22 and len(set(taskids))==22,'PM backlog: expected 22 unique tickets')
check(all(t['status'] not in {'TESTED','ACCEPTED'} for t in tickets) if backlog['actual_tests_executed'] is False else True,
      'PM backlog: ticket must not claim TESTED/ACCEPTED before tests execute')
check(all(bool(t['acceptance']) for t in tickets),'PM backlog: missing ticket acceptance')
check(all(set(t['dependencies']).issubset(taskids) for t in tickets),'PM backlog: unknown dependency')
check('PROJECT_MANAGER_DELIVERY_PLAYBOOK.md' in [x.name for x in ROOT.iterdir()],'PM playbook missing')
print('PM backlog:',len(tickets),'tickets; dependencies and acceptance checked')
print('Fixture schema and semantic checks complete.')
if fail:
 print('\nVALIDATION FAILED:')
 for item in fail:print(' -',item)
 sys.exit(1)
print('PASS — structural references, endpoints, fixtures, scenario catalog and AI evidence negative fixture')
