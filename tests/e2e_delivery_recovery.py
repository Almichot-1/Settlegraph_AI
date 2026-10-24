#!/usr/bin/env python3
"""Fault-inject a lost provider delivery, then verify automatic local recovery."""
import json
import subprocess
import time
import uuid


def compose(*args):
    return subprocess.run(['docker', 'compose', *args], check=True, capture_output=True, text=True).stdout.strip()


def sql(service, user, database, statement):
    return compose('exec', '-T', service, 'psql', '-At', '-v', 'ON_ERROR_STOP=1',
                   '-U', user, '-d', database, '-c', statement)


run_id, bank_run = sql('product-db', 'settlegraph', 'settlegraph',
    "SELECT id::text||'|'||banksim_run_id::text FROM scenario_runs "
    "WHERE scenario_key='normal_flow' AND status='COMPLETED' ORDER BY created_at DESC LIMIT 1").split('|')
event_id = str(uuid.uuid4())
reference = 'SIM-RECOVERY-' + event_id
compose('stop', 'banksim')
try:
    sequence = int(sql('banksim-db', 'banksim', 'banksim',
        "SELECT COALESCE(MAX(sequence),0)+1 FROM sim_events WHERE provider_id='BANK_A'"))
    event = {'event_id': event_id, 'provider_id': 'BANK_A', 'scenario_run_id': bank_run,
             'sequence': sequence, 'event_type': 'PAYMENT_ACCEPTED',
             'occurred_at': '2026-10-24T10:00:00Z', 'transfer_reference': reference,
             'payment_reference': reference, 'order_reference': 'ORD-RECOVERY-' + event_id,
             'amount': {'minor_units': '1', 'currency': 'ETB'}, 'payload_version': 1}
    payload = json.dumps(event, separators=(',', ':'))
    sql('banksim-db', 'banksim', 'banksim',
        f"INSERT INTO sim_events(id,scenario_id,provider_id,sequence,payload,occurred_at) "
        f"VALUES('{event_id}','{bank_run}','BANK_A',{sequence},'{payload}'::jsonb,"
        "'2026-10-24T10:00:00Z')")
    sql('product-db', 'settlegraph', 'settlegraph',
        "INSERT INTO source_status_snapshots(organization_id,provider_id,source_status,scenario_run_id) "
        f"VALUES('11111111-1111-4111-8111-111111111111','BANK_A','STALE','{run_id}')")
finally:
    compose('start', 'banksim')

for _ in range(40):
    delivery = sql('banksim-db', 'banksim', 'banksim',
        f"SELECT delivery_status FROM sim_events WHERE id='{event_id}'")
    evidence = sql('product-db', 'settlegraph', 'settlegraph',
        f"SELECT count(*) FROM inbox_events WHERE provider_event_id='{event_id}'")
    coverage = sql('product-db', 'settlegraph', 'settlegraph',
        "SELECT source_status FROM source_status_snapshots "
        f"WHERE scenario_run_id='{run_id}' AND provider_id='BANK_A' "
        "ORDER BY observed_at DESC,id DESC LIMIT 1")
    job = sql('product-db', 'settlegraph', 'settlegraph',
        "SELECT count(*) FROM jobs WHERE job_kind='RECONCILE' AND status='COMPLETED' "
        f"AND job_key LIKE 'coverage:{run_id}:BANK_A:%'")
    cursor = sql('product-db', 'settlegraph', 'settlegraph',
        "SELECT COALESCE(cursor_value,'0') FROM provider_sources "
        "WHERE organization_id='11111111-1111-4111-8111-111111111111' AND id='BANK_A'")
    if delivery == 'DELIVERED' and evidence == '1' and coverage == 'HEALTHY' and int(job) >= 1 and int(cursor) >= sequence:
        print('PASS: pending provider event redelivered; inbox, coverage and reconciliation recovered')
        break
    time.sleep(0.5)
else:
    raise AssertionError(f'recovery incomplete: {delivery}, inbox={evidence}, coverage={coverage}, jobs={job}, cursor={cursor}')
