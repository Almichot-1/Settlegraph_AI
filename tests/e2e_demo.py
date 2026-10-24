#!/usr/bin/env python3
"""Local Compose replay of the four synthetic scenarios and the review path."""
import json
import hmac
import hashlib
import secrets
import subprocess
import time
import uuid
import urllib.error
import urllib.request
from pathlib import Path

API = 'http://127.0.0.1:8080'
BANK = 'http://127.0.0.1:8082'
ORG = '11111111-1111-4111-8111-111111111111'
ENV = dict(line.split('=', 1) for line in Path('.env').read_text().splitlines() if '=' in line)

def request(base, path, method='GET', body=None, headers=None):
    h = {'Content-Type': 'application/json', **(headers or {})}
    req = urllib.request.Request(base + path, data=json.dumps(body).encode() if body is not None else None,
                                 headers=h, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            return response.status, json.load(response), response.headers
    except urllib.error.HTTPError as error:
        return error.code, json.load(error), error.headers

def expect(condition, message):
    if not condition:
        raise AssertionError(message)

status, session, _ = request(API, '/v1/demo/sessions', 'POST', {'display_name':'E2E Reviewer'},
                             {'X-Demo-Access-Code':ENV['DEMO_ACCESS_CODE']})
expect(status == 201, f'demo session: {status}')
HEADERS = {'Authorization':'Bearer ' + session['access_token'], 'X-Organization-Id':ORG}
PROVIDER = {'Authorization':'Bearer ' + ENV['BANKSIM_PROVIDER_TOKEN']}
EVALUATOR = {'Authorization':'Bearer ' + ENV['BANKSIM_EVALUATOR_TOKEN']}

def api(path, method='GET', body=None, key=None, match=None):
    headers = dict(HEADERS)
    if key: headers['Idempotency-Key'] = key
    if match: headers['If-Match'] = match
    return request(API, path, method, body, headers)

def wait_for(predicate, label):
    for _ in range(40):
        result = predicate()
        if result: return result
        time.sleep(0.25)
    raise AssertionError('timed out waiting for ' + label)

def all_items(path):
    items = []
    cursor = None
    while True:
        query = '?limit=100'
        if cursor is not None:
            query += '&cursor=' + cursor
        status, page, _ = api(path + query)
        expect(status == 200, f'{path}: {status}')
        items.extend(page['items'])
        cursor = page['next_cursor']
        if cursor is None:
            return items

def scenario(key, seed):
    nonce = secrets.token_hex(8)
    status, run, _ = api(f'/v1/demo/scenarios/{key}/runs', 'POST', {'seed':seed}, nonce+'-create')
    expect(status == 201, f'{key} create: {status} {run}')
    return run, nonce

def advance(run, nonce, step):
    status, updated, _ = api(f"/v1/demo/scenario-runs/{run['id']}/advance", 'POST',
                             {'expected_step':step}, f'{nonce}-step-{step}')
    expect(status == 200 and updated['step'] == step+1, f'{run["scenario_key"]} step {step}: {status} {updated}')
    return updated

def observations(path, run_id):
    return [item for item in all_items(path) if item.get('scenario_run_id') == run_id]

def runs(run_id):
    return [item for item in all_items('/v1/reconciliation/runs') if item.get('scenario_run_id') == run_id]

def cases(run_id):
    return [item for item in all_items('/v1/cases') if item.get('scenario_run_id') == run_id]

def provider_event(order_reference):
    cursor = None
    while True:
        path = '/provider/v1/institutions/BANK_A/events'
        if cursor is not None:
            path += '?cursor=' + cursor
        status, page, _ = request(BANK, path, headers=PROVIDER)
        expect(status == 200, 'provider event lookup failed')
        for event in page['items']:
            if event.get('order_reference') == order_reference:
                return event
        cursor = page['next_cursor']
        if cursor is None:
            raise AssertionError('provider event missing for ' + order_reference)

base_seed = secrets.randbelow(1_000_000_000) + 1_000_000

normal, nonce = scenario('normal_flow', base_seed)
status, _, _ = api(f"/v1/demo/scenario-runs/{normal['id']}/advance", 'POST',
                   {'expected_step':1}, nonce+'-stale-step')
expect(status == 412, 'scenario advance accepted a stale expected_step')
advance(normal, nonce, 0)
advance(normal, nonce, 1)
wait_for(lambda: any(x['matched_payment_settlement'] == 1 for x in runs(normal['id'])), 'normal reconciliation')
expect(len(cases(normal['id'])) == 0, 'normal flow opened a case')
normal_repeat, repeat_nonce = scenario('normal_flow', base_seed)
advance(normal_repeat, repeat_nonce, 0)
advance(normal_repeat, repeat_nonce, 1)
wait_for(lambda: any(x['matched_payment_settlement'] == 1 for x in runs(normal_repeat['id'])), 'repeat reconciliation')
for path in ('/v1/transactions', '/v1/settlements'):
    first = observations(path, normal['id'])
    second = observations(path, normal_repeat['id'])
    expect(len(first) == len(second) == 1, f'{path} repeated scenario count changed')
    fields = ('provider_id', 'amount', 'occurred_at')
    expect(all(first[0][name] == second[0][name] for name in fields), f'{path} logical replay changed')
repeated_orders = [item for item in all_items('/v1/orders')
                   if item.get('scenario_run_id') in (normal['id'], normal_repeat['id'])]
expect(len(repeated_orders) == 2 and all(x['status'] == 'RECONCILED' for x in repeated_orders),
       'same-seed orders were not scoped to their runs')

delayed, nonce = scenario('delayed_settlement', base_seed+1)
advance(delayed, nonce, 0)
advance(delayed, nonce, 1)
case = wait_for(lambda: next((x for x in cases(delayed['id']) if x['status'] == 'OPEN'), None), 'delayed case')
historical = wait_for(lambda: next((x for x in runs(delayed['id']) if x['unmatched_count'] == 1), None), 'historical missing run')
status, evidence, _ = api(f"/v1/cases/{case['id']}/evidence")
expect(status == 200 and len(evidence['items']) >= 2, 'case evidence missing')
status, investigation, _ = api(f"/v1/cases/{case['id']}/investigations", 'POST',
                                {'mode':'EVIDENCE_ANALYSIS'}, nonce+'-investigate')
expect(status == 202 and investigation['provider'] == 'OFFLINE_STUB', 'offline investigation failed')
allowed = {item['id'] for item in evidence['items']}
expect(set(investigation['result']['evidence_ids']) <= allowed, 'investigation fabricated evidence')
advance(delayed, nonce, 2)
updated_case = wait_for(lambda: next((x for x in cases(delayed['id']) if x['status'] == 'EVIDENCE_UPDATED'), None), 'late settlement evidence')
wait_for(lambda: any(x['matched_payment_settlement'] == 1 for x in runs(delayed['id'])), 'later settlement run')
expect(any(x['id'] == historical['id'] and x['unmatched_count'] == 1 for x in runs(delayed['id'])),
       'later reconciliation rewrote a historical run')
status, _, stale_etag = api(f"/v1/cases/{case['id']}")
expect(status == 200, 'case detail failed')
status, decision, _ = api(f"/v1/cases/{case['id']}/decisions", 'POST',
                           {'decision':'RESOLVE','reason':'Reviewed observed settlement.'}, nonce+'-decision', stale_etag.get('ETag'))
expect(status == 200 and decision['status'] == 'RESOLVED', 'review decision failed')
status, retry, _ = api(f"/v1/cases/{case['id']}/decisions", 'POST',
                        {'decision':'RESOLVE','reason':'Reviewed observed settlement.'}, nonce+'-decision', stale_etag.get('ETag'))
expect(status == 200 and retry['version'] == decision['version'], 'decision retry changed version')
status, _, _ = api(f"/v1/cases/{case['id']}/decisions", 'POST',
                    {'decision':'REOPEN','reason':'Need another review.'}, nonce+'-stale', stale_etag.get('ETag'))
expect(status == 412, 'stale case version was accepted')

duplicate, nonce = scenario('duplicate_webhook', base_seed+2)
advance(duplicate, nonce, 0)
advance(duplicate, nonce, 1)
wait_for(lambda: len(observations('/v1/settlements', duplicate['id'])) == 1, 'duplicate settlement')
expect(len(observations('/v1/transactions', duplicate['id'])) == 1, 'duplicate webhook created extra payment')
original = provider_event(f'ORD-{base_seed+2}')
altered = {key: original[key] for key in ('event_id', 'provider_id', 'scenario_run_id',
           'sequence', 'occurred_at', 'transfer_reference', 'order_reference', 'payment_reference', 'amount')}
altered['event_type'] = original['type']
altered['payload_version'] = 1
altered['amount'] = {'minor_units': str(int(original['amount']['minor_units']) + 1), 'currency': 'ETB'}
stamp = str(int(time.time()))
raw = json.dumps(altered).encode()
signature = hmac.new(ENV['BANKSIM_WEBHOOK_SECRET'].encode(),
                     stamp.encode()+b'.'+altered['event_id'].encode()+b'.'+raw, hashlib.sha256).hexdigest()
headers = {'X-BankSim-Event-Id':altered['event_id'], 'X-BankSim-Delivery-Timestamp':stamp,
           'X-BankSim-Signature':'v1='+signature}
status, _, _ = request(API, '/v1/integrations/banksim/events', 'POST', altered, headers)
expect(status == 409, 'event ID accepted with changed signed content')

outage, nonce = scenario('provider_outage', base_seed+3)
advance(outage, nonce, 0)
advance(outage, nonce, 1)
expect(len(observations('/v1/settlements', outage['id'])) == 0, 'outage exposed settlement early')
expect(len(cases(outage['id'])) == 0, 'outage opened false missing-settlement case')
bank_run_id = provider_event(f'ORD-{base_seed+3}')['scenario_run_id']
status, truth, _ = request(BANK, f'/evaluator/v1/runs/{bank_run_id}/truth', headers=EVALUATOR)
expect(status == 200 and truth['expected_settlements'] == 1 and truth['source_ledger_balanced'],
       'provider outage changed private ledger truth')
advance(outage, nonce, 2)
wait_for(lambda: len(observations('/v1/settlements', outage['id'])) == 1, 'outage recovery')
expect(len(cases(outage['id'])) == 0, 'outage recovery opened false case')

fake_id = '77777777-7777-4777-8777-777777777777'
payload = {'event_id':fake_id,'provider_id':'BANK_A','scenario_run_id':fake_id,'sequence':1,
           'event_type':'PAYMENT_ACCEPTED','occurred_at':'2026-10-24T09:00:00Z',
           'transfer_reference':'SIM-TEST','order_reference':'ORD-TEST','payment_reference':'SIM-TEST',
           'amount':{'minor_units':'250000','currency':'ETB'},'payload_version':1}
raw = json.dumps(payload).encode()
stamp = str(int(time.time()))
signature = hmac.new(ENV['BANKSIM_WEBHOOK_SECRET'].encode(),stamp.encode()+b'.'+fake_id.encode()+b'.'+raw,hashlib.sha256).hexdigest()
headers = {'X-BankSim-Event-Id':fake_id,'X-BankSim-Delivery-Timestamp':stamp,
           'X-BankSim-Signature':'v1=' + '0'*64}
status, _, _ = request(API,'/v1/integrations/banksim/events','POST',payload,headers)
expect(status == 401, 'invalid HMAC was accepted')
headers['X-BankSim-Signature'] = 'v1=' + signature
headers['X-BankSim-Delivery-Timestamp'] = str(int(time.time())-600)
status, _, _ = request(API,'/v1/integrations/banksim/events','POST',payload,headers)
expect(status == 401, 'expired delivery timestamp was accepted')

replay_id = str(uuid.uuid4())
replay_ref = 'SIM-E2E-' + secrets.token_hex(8)
replay = {'event_id':replay_id, 'provider_id':'BANK_A',
          'scenario_run_id':provider_event(f'ORD-{base_seed}')['scenario_run_id'],
          'sequence':999999999, 'event_type':'PAYMENT_ACCEPTED',
          'occurred_at':'2026-10-26T09:00:00Z', 'transfer_reference':replay_ref,
          'order_reference':'ORD-E2E-REPLAY', 'payment_reference':replay_ref,
          'amount':{'minor_units':'1','currency':'ETB'}, 'payload_version':1}
stamp = str(int(time.time()))
raw = json.dumps(replay).encode()
signature = hmac.new(ENV['BANKSIM_WEBHOOK_SECRET'].encode(),
                     stamp.encode()+b'.'+replay_id.encode()+b'.'+raw, hashlib.sha256).hexdigest()
headers = {'X-BankSim-Event-Id':replay_id, 'X-BankSim-Delivery-Timestamp':stamp,
           'X-BankSim-Signature':'v1='+signature}
prior_run_count = len(runs(normal['id']))
subprocess.run(['docker', 'compose', 'stop', 'worker'], check=True, stdout=subprocess.DEVNULL)
try:
    for attempt in range(10):
        status, result, _ = request(API, '/v1/integrations/banksim/events', 'POST', replay, headers)
        expect(status == 202 and result['duplicate'] == (attempt > 0), 'ten-delivery replay failed')
finally:
    subprocess.run(['docker', 'compose', 'start', 'worker'], check=True, stdout=subprocess.DEVNULL)
expect(sum(item['external_reference'] == replay_ref for item in all_items('/v1/transactions')) == 1,
       'ten deliveries created more than one transaction')
wait_for(lambda: len(runs(normal['id'])) > prior_run_count, 'outbox replay after worker restart')

print('PASS: four scenarios, deterministic replay, historical runs, review, restart dedup, webhook rejection')
