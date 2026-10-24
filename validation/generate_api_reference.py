#!/usr/bin/env python3
from pathlib import Path
import yaml
ROOT=Path(__file__).resolve().parents[1]; C=ROOT/'contracts'
D=yaml.safe_load((C/'settlegraph-public.openapi.yaml').read_text())
B=yaml.safe_load((C/'banksim-internal.openapi.yaml').read_text())
lines=['# Complete HTTP endpoint inventory and implementation schedule','',
'**Status:** Proposed contract; no endpoints are implemented by this document. The YAML files under `contracts/` are normative for request/response schemas, error payloads, security and parameter details.','',
'**Priority key:** P0=first end-to-end demo; P1=rich product; P2=CSV import fallback. Web and Flutter can mock P1/P2 responses until implemented.','']
for source,doc in [('Product API',D),('BankSim internal simulator',B)]:
 lines.append(f'## {source}')
 lines.append('')
 tags=[]
 for path,methods in doc['paths'].items():
  for method,o in methods.items():
   tag=o['tags'][0]
   if tag not in tags:tags.append(tag)
 for tag in tags:
  lines.append(f'### {tag}')
  lines.append('')
  for path,methods in doc['paths'].items():
   for method,o in methods.items():
    if o['tags'][0]!=tag:continue
    successes=[code for code in o['responses'] if str(code).startswith('2')]
    priority=o.get('x-implementation-priority','-')
    lines.append(f'**{method.upper()} `{path}`** — {o["summary"]}  ')
    lines.append(f'Operation: `{o["operationId"]}` · Success: `{", ".join(successes)}` · Priority: **{priority}**')
    names=[]
    for par in o.get('parameters',[]):
     if '$ref' in par:par=doc['components']['parameters'][par['$ref'].split('/')[-1]]
     names.append(f'{par["name"]}{"*" if par.get("required") else ""} ({par["in"]})')
    if names:lines.append('Parameters: '+', '.join(names))
    request=o.get('requestBody',{}).get('content',{}).get('application/json',{}).get('schema')
    if request:
     model=request.get('$ref','inline').split('/')[-1]
     lines.append(f'Request JSON schema: `{model}`')
    content=o['responses'].get(successes[0],{}).get('content',{}).get('application/json',{}) if successes else {}
    response=content.get('schema',{})
    if response:lines.append(f'Response JSON schema: `{response.get("$ref","inline").split("/")[-1]}`')
    lines.append('')
lines += ['## Required shared headers','',
'- `Authorization: Bearer <JWT>` on product routes except `/health/*`, demo session creation and signed webhook.',
'- `X-Organization-Id: <uuid>` on tenant-scoped product routes.',
'- `Idempotency-Key: <stable key>` on documented mutating operations.',
'- `If-Match: "<integer>"` on optimistic case mutations.',
'- Webhook only: `X-BankSim-Event-Id`, `X-BankSim-Delivery-Timestamp` and `X-BankSim-Signature`. HMAC requires exact raw-body bytes.',
'', '## Shared data and errors','',
'- Money example: `{"minor_units":"250000","currency":"ETB"}` = ETB 2,500.00.',
'- Timestamp example: `2026-10-24T09:00:00Z`.',
'- Error: `{"error":{"code":"VERSION_CONFLICT","message":"...","request_id":"...","details":{}}}`.',
'- Tenant-scoped read on foreign ID: 404 to avoid enumeration.',
'- Webhook duplicates ACK with HTTP 202 and `{"accepted":true,"duplicate":true}`.',
'- No live-money or real-bank guarantees: all BankSim amounts are simulated.',
'']
(ROOT/'API_REFERENCE.md').write_text('\n'.join(lines))
print('Generated reference:',len(lines),'lines')
