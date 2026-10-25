#!/usr/bin/env python3
"""Run native review, optionally coordinating a browser-created case.

Credentials stay in process memory; artifacts contain synthetic resource IDs,
response codes and versions only. No access code is compiled into the app.
"""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import argparse
import hashlib
import json
import secrets
import subprocess
import sys
import threading
import time
import urllib.request
import urllib.parse
import uuid

root=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(root/'scripts'))
from mobile_env import environment
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--device',default='emulator-5554')
parser.add_argument('--api-url',default='http://127.0.0.1:8080')
parser.add_argument('--device-api-url',default='http://10.0.2.2:8080')
parser.add_argument('--handoff',type=Path)
parser.add_argument('--coordination-dir',type=Path)
parser.add_argument('--artifacts',type=Path,default=root/'verification/native')
args=parser.parse_args()
flutter,process_env=environment()
settings=dict(line.split('=',1) for line in (root/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
trace=[]
decision_attempts=[]
unconfirmed=False
financial_before=None
args.artifacts.mkdir(parents=True,exist_ok=True)
def request(path,body=None,headers=None):
    req=urllib.request.Request(args.api_url+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json',**(headers or {})})
    with urllib.request.urlopen(req,timeout=20) as response:
        if path!='/v1/demo/sessions':trace.append({'method':req.method,'path':path,'status':response.status})
        return json.load(response)
def login(code):return request('/v1/demo/sessions',{'display_name':'Integration reviewer'},{'X-Demo-Access-Code':code})
owner=login(settings['DEMO_ACCESS_CODE']);session=login(settings['DEMO_REVIEWER_ACCESS_CODE'])
if session['user']['id']==owner['user']['id']:raise SystemExit('Native review requires a distinct reviewer identity')
headers={'Authorization':'Bearer '+owner['access_token'],'X-Organization-Id':owner['default_organization_id']}
def records(path):
    items=[];cursor=None;seen=set()
    for _ in range(100):
        page=request(path+('&' if '?' in path else '?')+urllib.parse.urlencode({'limit':100,**({'cursor':cursor} if cursor else {})}),headers=headers)
        items.extend(page['items']);cursor=page['next_cursor']
        if not cursor:return items
        if cursor in seen:raise RuntimeError('Repeated API cursor')
        seen.add(cursor)
    raise RuntimeError('API history exceeded its test bound')
def financial_snapshot():
    return {'transactions':records('/v1/transactions?scenario_run_id='+run['id']),
            'settlements':[item for item in records('/v1/settlements') if item.get('scenario_run_id')==run['id']]}
def post(path,body,version=None):
    return request(path,body,{**headers,'Idempotency-Key':str(uuid.uuid4()),**({'If-Match':'"'+str(version)+'"'} if version is not None else {})})
if args.handoff:
    handoff=json.loads(args.handoff.read_text())
    for key in ['scenario_run_id','case_id']:uuid.UUID(handoff[key])
    run=request('/v1/demo/scenario-runs/'+handoff['scenario_run_id'],headers=headers)
    case=request('/v1/cases/'+handoff['case_id'],headers=headers)
    if case['scenario_run_id']!=run['id'] or case['assigned_to']!=session['user']['id']:raise SystemExit('Browser handoff does not match the reviewer or scenario')
else:
    run=post('/v1/demo/scenarios/delayed_settlement/runs',{'seed':secrets.randbelow(1000000)+1000})
    for step in [0,1]:post('/v1/demo/scenario-runs/'+run['id']+'/advance',{'expected_step':step})
    for _ in range(80):
        page=request('/v1/cases?scenario_run_id='+run['id'],headers=headers)
        if page['items']:case=page['items'][0];break
        time.sleep(.25)
    else:raise SystemExit('Delayed case did not appear')
    case=post('/v1/cases/'+case['id']+'/assign',{'assignee_id':session['user']['id']},case['version'])
    analysis=post('/v1/cases/'+case['id']+'/investigations',{'mode':'EVIDENCE_ANALYSIS'})
    for _ in range(80):
        current=request('/v1/cases/'+case['id']+'/investigations/'+analysis['id'],headers=headers)
        if current['status']=='COMPLETED':break
        if current['status'] in ['FAILED','REJECTED']:raise SystemExit('Investigation failed before native review')
        time.sleep(.25)
    else:raise SystemExit('Investigation did not complete')
    case=request('/v1/cases/'+case['id'],headers=headers)
nonce=secrets.token_hex(24)
stopping=threading.Event()
class Harness(BaseHTTPRequestHandler):
    released=False
    lock=threading.Lock()
    def log_message(self,*unused):pass
    def do_POST(self):
        if self.path!='/'+nonce+'/attempt':self.send_error(404);return
        length=int(self.headers.get('Content-Length','0'))
        if not 0<length<=8192:self.send_error(413);return
        attempt=json.loads(self.rfile.read(length))
        if set(attempt)!=set(['body','headers','status']) or set(attempt['headers'])!=set(['If-Match','Idempotency-Key']):self.send_error(400);return
        decision_attempts.append(attempt)
        self.send_response(200);self.send_header('Content-Length','2');self.end_headers();self.wfile.write(b'{}')
    def do_GET(self):
        global unconfirmed,financial_before
        if self.path=='/'+nonce+'/session':
            body={'session':session,'case':case,'scenario_run_id':run['id']}
        elif self.path=='/'+nonce+'/settlement':
            with Harness.lock:
                if not Harness.released:
                    if args.coordination_dir:
                        args.coordination_dir.mkdir(parents=True,exist_ok=True)
                        (args.coordination_dir/'settlement-requested.json').write_text(json.dumps({'scenario_run_id':run['id'],'case_id':case['id']}))
                        deadline=time.monotonic()+120
                        while not (args.coordination_dir/'settlement-released.json').exists():
                            if stopping.is_set() or time.monotonic()>deadline:self.send_error(503,'Browser did not release settlement');return
                            time.sleep(.1)
                    else:post('/v1/demo/scenario-runs/'+run['id']+'/advance',{'expected_step':2})
                    deadline=time.monotonic()+30
                    while request('/v1/cases/'+case['id'],headers=headers)['status']!='EVIDENCE_UPDATED':
                        if time.monotonic()>deadline:self.send_error(503,'Late evidence was not observed');return
                        time.sleep(.25)
                    financial_before=financial_snapshot()
                    Harness.released=True
            body={'released':True}
        elif self.path in ['/'+nonce+'/completed','/'+nonce+'/unconfirmed']:
            if self.path.endswith('/unconfirmed'):unconfirmed=True
            name='review-unconfirmed.png' if self.path.endswith('/unconfirmed') else 'review-completed.png'
            with (args.artifacts/name).open('wb') as output:
                subprocess.run(['adb','-s',args.device,'exec-out','screencap','-p'],env=process_env,stdout=output,check=True,timeout=15)
            body={'captured':True}
        else:self.send_error(404);return
        payload=json.dumps(body).encode();self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Cache-Control','no-store');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
server=ThreadingHTTPServer(('127.0.0.1',0),Harness);server.daemon_threads=True
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
    command=[flutter,'--no-version-check','test','integration_test/app_test.dart','-d',args.device,'--dart-define=LIVE_HARNESS=http://10.0.2.2:'+str(server.server_port)+'/'+nonce,'--dart-define=API_BASE_URL='+args.device_api_url,'--timeout','5m','--reporter','expanded']
    for phase in ['submit','retry']:
        result=subprocess.run([*command,'--dart-define=RETRY_PHASE='+phase],cwd=root/'mobile_app',env=process_env)
        if result.returncode:raise SystemExit(result.returncode)
        if phase=='submit':
            if not unconfirmed:raise SystemExit('Native UI did not retain an unconfirmed decision')
            subprocess.run(['adb','-s',args.device,'shell','am','force-stop','dev.settlegraph.settlegraph_mobile'],env=process_env,check=True)
            running=subprocess.run(['adb','-s',args.device,'shell','pidof','dev.settlegraph.settlegraph_mobile'],env=process_env,capture_output=True,text=True)
            if running.stdout.strip():raise SystemExit('Native app process did not stop')
            print('Native process stopped; next installation must restore the encrypted pending envelope.',flush=True)
    final=request('/v1/cases/'+case['id'],headers=headers)
    events=request('/v1/cases/'+case['id']+'/events?limit=100',headers=headers)
    resolutions=[e for e in events['items'] if e['type']=='RESOLVED']
    if final['status']!='RESOLVED' or len(resolutions)!=1 or resolutions[0]['actor']!='demo-reviewer':raise SystemExit('Native reviewer decision did not match product history')
    audit=[entry for entry in records('/v1/audit-events') if entry['resource_type']=='CASE' and entry['resource_id']==case['id'] and entry['action']=='RESOLVED']
    if len(audit)!=1 or audit[0]['actor']!='demo-reviewer':raise SystemExit('Native retry duplicated or misattributed the decision audit')
    if financial_before is None or financial_snapshot()!=financial_before:raise SystemExit('Human review changed financial source observations')
    if [item['status'] for item in decision_attempts]!=[412,200,200] or decision_attempts[1]!=decision_attempts[2]:raise SystemExit('Native retry did not preserve the exact body, key and version after process termination')
    artifact={'scenario_run_id':run['id'],'case_id':case['id'],'web_user_id':owner['user']['id'],'mobile_user_id':session['user']['id'],'actor':resolutions[0]['actor'],'final_status':final['status'],'final_version':final['version'],'event_count':len(events['items']),'completed_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'responses':trace,'decision_attempts':decision_attempts,'process_restart_verified':True,'resolution_audit_count':len(audit),'financial_sources_unchanged':True,'financial_source_hash':hashlib.sha256(json.dumps(financial_before,sort_keys=True).encode()).hexdigest()}
    (args.artifacts/'trace.json').write_text(json.dumps(artifact,indent=2))
    print('PASS: native reviewer and product case/audit history agree')
finally:
    stopping.set();server.shutdown();server.server_close()
