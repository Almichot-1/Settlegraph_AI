#!/usr/bin/env python3
"""Run two complete browser/native rehearsals on fresh isolated demo databases.

Only newly created settlegraph-test-* Compose projects may be reset. The normal
local demo and its volumes are never reset by this harness.
"""
import argparse
from datetime import datetime,timezone
import json
import os
from pathlib import Path
import socket
import subprocess
import uuid
from mobile_env import ROOT,environment

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--runs',type=int,default=2)
p.add_argument('--device',default='emulator-5554')
a=p.parse_args()
if not 1<=a.runs<=3:p.error('--runs must be 1 to 3')
_,base_env=environment()
subprocess.run(['adb','-s',a.device,'get-state'],env=base_env,check=True,capture_output=True)
subprocess.run(['python3','validation/init_demo_env.py'],cwd=ROOT,check=True)
base=ROOT/'verification'/('acceptance-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ'))
base.mkdir(parents=True,exist_ok=True)
used=set()
def port():
 while True:
  with socket.socket() as s:s.bind(('127.0.0.1',0));value=s.getsockname()[1]
  if value not in used:used.add(value);return value
summary=[]
for rehearsal in range(1,a.runs+1):
 project='settlegraph-test-'+uuid.uuid4().hex[:10]
 assert project.startswith('settlegraph-test-') and len(project)>20
 artifacts=base/str(rehearsal);artifacts.mkdir()
 env=base_env.copy();env.update(COMPOSE_PROJECT_NAME=project,COMPOSE_FILE=str(ROOT/'compose.yaml')+os.pathsep+str(ROOT/'compose.web.yaml'),API_PORT=str(port()),BANKSIM_PORT=str(port()),WEB_PORT=str(port()))
 env.update(SETTLEGRAPH_API_URL='http://127.0.0.1:'+env['API_PORT'],SETTLEGRAPH_BANK_URL='http://127.0.0.1:'+env['BANKSIM_PORT'],SETTLEGRAPH_WEB_URL='http://127.0.0.1:'+env['WEB_PORT'],ANDROID_API_URL='http://10.0.2.2:'+env['API_PORT'],ANDROID_DEVICE=a.device,CROSS_CLIENT='1',CROSS_CLIENT_ARTIFACTS=str(artifacts))
 command=['docker','compose','-p',project,'-f',str(ROOT/'compose.yaml'),'-f',str(ROOT/'compose.web.yaml')]
 try:
  print(f'Starting isolated fresh-database rehearsal {rehearsal}/{a.runs}',flush=True)
  subprocess.run(['python3','scripts/local_stack.py','up'],cwd=ROOT,env=env,check=True)
  subprocess.run([str(ROOT/'.venv/bin/python'),'tests/contract_runtime.py'],cwd=ROOT,env=env,check=True)
  subprocess.run(['npm','run','test:live'],cwd=ROOT/'settlegraph-web',env=env,check=True)
  trace=json.loads((artifacts/'trace.json').read_text())
  trace['rehearsal']=rehearsal;summary.append(trace)
 finally:
  with (artifacts/'services.log').open('w') as log:
   subprocess.run([*command,'logs','--no-color','--tail','100'],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT)
  subprocess.run([*command,'down','--volumes'],cwd=ROOT,env=env,check=True)
revision=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
dirty=bool(subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True).strip())
report={'completed_at':datetime.now(timezone.utc).isoformat(),'tested_commit':revision,'working_tree_dirty':dirty,'native_client':'Android Flutter','rehearsals':summary,'aws':'NOT_RUN','ios':'NOT_RUN (Linux host)'}
(base/'summary.json').write_text(json.dumps(report,indent=2))
(ROOT/'verification/latest-acceptance.json').write_text(json.dumps(report,indent=2))
print('PASS: two-client shared-case acceptance; artifacts:',base)
