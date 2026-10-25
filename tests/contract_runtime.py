#!/usr/bin/env python3
"""Validate live demo JSON responses against the public OpenAPI schemas."""
import json
import os
import urllib.request
from pathlib import Path

import yaml
from jsonschema import Draft4Validator, FormatChecker

BASE = os.environ.get('SETTLEGRAPH_API_URL', 'http://127.0.0.1:8080')
ENV = dict(line.split('=', 1) for line in Path('.env').read_text().splitlines() if '=' in line)
DOC = yaml.safe_load(Path('contracts/settlegraph-public.openapi.yaml').read_text())


def convert(value):
    if isinstance(value, list):
        return [convert(item) for item in value]
    if isinstance(value, dict):
        result = {key: convert(item) for key, item in value.items()
                  if key not in ('example', 'deprecated', 'readOnly', 'writeOnly')}
        if '$ref' in result:
            result['$ref'] = result['$ref'].replace('#/components/schemas/', '#/definitions/')
        if result.pop('nullable', False):
            return {'anyOf': [result, {'type': 'null'}]}
        return result
    return value


DEFINITIONS = {name: convert(schema) for name, schema in DOC['components']['schemas'].items()}


def request(path, headers=None, body=None):
    payload = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=payload, headers=headers or {})
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.load(response)


def verify(template, path, headers, method='get', status='200', body=None):
    result = request(path, headers, body)
    response = DOC['paths'][template][method]['responses'][status]['content']['application/json']['schema']
    schema = {**convert(response), 'definitions': DEFINITIONS}
    errors = list(Draft4Validator(schema, format_checker=FormatChecker()).iter_errors(result))
    if errors:
        raise AssertionError(f'{template}: {errors[0]}')
    return result


session = verify('/v1/demo/sessions', '/v1/demo/sessions',
                 {'Content-Type': 'application/json', 'X-Demo-Access-Code': ENV['DEMO_ACCESS_CODE']},
                 'post', '201', {'display_name': 'Runtime Contract Check'})
headers = {'Authorization': 'Bearer ' + session['access_token'],
           'X-Organization-Id': session['default_organization_id']}
paths = ['/v1/me', '/v1/members', '/v1/providers', '/v1/accounts', '/v1/dashboard/summary',
         '/v1/dashboard/series', '/v1/orders', '/v1/transactions', '/v1/settlements',
         '/v1/reconciliation/runs', '/v1/cases', '/v1/imports', '/v1/audit-events',
         '/v1/demo/scenarios']
results = {path: verify(path, path + ('?from=2026-10-24T00:00:00Z&to=2026-10-25T00:00:00Z' if path == '/v1/dashboard/series' else ''), headers) for path in paths}
for collection, template in [('/v1/orders', '/v1/orders/{id}'),
                             ('/v1/transactions', '/v1/transactions/{id}'),
                             ('/v1/settlements', '/v1/settlements/{id}'),
                             ('/v1/reconciliation/runs', '/v1/reconciliation/runs/{runId}'),
                             ('/v1/cases', '/v1/cases/{caseId}')]:
    if results[collection]['items']:
        verify(template, collection + '/' + results[collection]['items'][0]['id'], headers)
if results['/v1/cases']['items']:
    case_id = results['/v1/cases']['items'][0]['id']
    for suffix, template in [('/evidence', '/v1/cases/{caseId}/evidence'),
                             ('/events', '/v1/cases/{caseId}/events'),
                             ('/investigations', '/v1/cases/{caseId}/investigations')]:
        verify(template, '/v1/cases/' + case_id + suffix, headers)
if results['/v1/reconciliation/runs']['items']:
    run = results['/v1/reconciliation/runs']['items'][0]
    matches = verify('/v1/reconciliation/runs/{runId}/matches',
                     '/v1/reconciliation/runs/' + run['id'] + '/matches', headers)
    if matches['items']:
        verify('/v1/reconciliation/proofs/{id}',
               '/v1/reconciliation/proofs/' + matches['items'][0]['id'], headers)
    if run.get('scenario_run_id'):
        verify('/v1/demo/scenario-runs/{scenarioRunId}',
               '/v1/demo/scenario-runs/' + run['scenario_run_id'], headers)
print('PASS: live public read responses match OpenAPI schemas')
