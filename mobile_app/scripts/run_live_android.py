#!/usr/bin/env python3
"""Run native mobile review against local BankSim through the public product API.

The host reads its ignored .env. The device receives a short-lived demo session
from a nonce-scoped loopback harness; no access code or signing key is compiled.
"""
from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import argparse
import json
import secrets
import subprocess
import threading
import time
import urllib.request
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--device', default='emulator-5554')
parser.add_argument('--api-url', default='http://127.0.0.1:8080')
parser.add_argument('--device-api-url', default='http://10.0.2.2:8080')
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
env = dict(line.split('=', 1) for line in (root / '.env').read_text().splitlines() if '=' in line and not line.startswith('#'))

def request(path, body=None, headers=None):
    req = urllib.request.Request(args.api_url + path,
        data=None if body is None else json.dumps(body).encode(),
        headers={'Content-Type': 'application/json', **(headers or {})})
    with urllib.request.urlopen(req, timeout=20) as response:
        return json.load(response)

session = request('/v1/demo/sessions', {'display_name': 'Mobile integration reviewer'}, {'X-Demo-Access-Code': env['DEMO_ACCESS_CODE']})
headers = {'Authorization': 'Bearer ' + session['access_token'], 'X-Organization-Id': session['default_organization_id']}
def post(path, body, match=None):
    return request(path, body, {**headers, 'Idempotency-Key': str(uuid.uuid4()), **({'If-Match': '"' + str(match) + '"'} if match else {})})
run = post('/v1/demo/scenarios/delayed_settlement/runs', {'seed': secrets.randbelow(1_000_000) + 1000})
for step in (0, 1):
    post('/v1/demo/scenario-runs/' + run['id'] + '/advance', {'expected_step': step})
case = None
for _ in range(80):
    page = request('/v1/cases?scenario_run_id=' + run['id'], headers=headers)
    if page['items']:
        case = page['items'][0]
        break
    time.sleep(.25)
if not case:
    raise SystemExit('Delayed-settlement case did not appear')
case = post('/v1/cases/' + case['id'] + '/assign', {'assignee_id': session['user']['id']}, case['version'])
post('/v1/cases/' + case['id'] + '/investigations', {'mode': 'EVIDENCE_ANALYSIS'})
nonce = secrets.token_hex(24)

class Harness(BaseHTTPRequestHandler):
    released = False
    def log_message(self, *unused):
        pass
    def do_GET(self):
        if self.path == '/' + nonce + '/session':
            body = {'session': session, 'case': case, 'scenario_run_id': run['id']}
        elif self.path == '/' + nonce + '/settlement':
            if not Harness.released:
                post('/v1/demo/scenario-runs/' + run['id'] + '/advance', {'expected_step': 2})
                Harness.released = True
            body = {'released': True}
        else:
            self.send_error(404)
            return
        payload = json.dumps(body).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Cache-Control', 'no-store')
        self.send_header('Content-Length', str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

server = ThreadingHTTPServer(('127.0.0.1', 0), Harness)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
try:
    result = subprocess.run(['flutter', 'test', 'integration_test/app_test.dart', '-d', args.device,
        '--dart-define=LIVE_HARNESS=http://10.0.2.2:' + str(server.server_port) + '/' + nonce,
        '--dart-define=API_BASE_URL=' + args.device_api_url,
        '--reporter', 'expanded'], cwd=root / 'mobile_app')
    if result.returncode:
        raise SystemExit(result.returncode)
    final = request('/v1/cases/' + case['id'], headers=headers)
    events = request('/v1/cases/' + case['id'] + '/events', headers=headers)
    if final['status'] != 'RESOLVED' or not any(e['type'] == 'RESOLVED' for e in events['items']):
        raise SystemExit('Native decision did not appear in the product API history')
    trace = {'scenario_run_id': run['id'], 'case_id': case['id'], 'user_id': session['user']['id'], 'final_status': final['status'], 'final_version': final['version'], 'event_count': len(events['items']), 'client': 'Android Flutter emulator', 'completed_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}
    (root / 'mobile_app' / 'verification').mkdir(exist_ok=True)
    (root / 'mobile_app' / 'verification' / 'native-live-trace.json').write_text(json.dumps(trace, indent=2))
    print('PASS: native Flutter decision verified independently through product API and case history')
finally:
    server.shutdown()
    server.server_close()
