#!/usr/bin/env python3
"""Run the mobile project's pinned SDK, optionally loading runtime demo codes."""
import argparse
import json
from pathlib import Path
import subprocess
from mobile_env import ROOT,environment
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--demo-env',action='store_true')
a,command=p.parse_known_args()
flutter,env=environment()
if a.demo_env:
 for line in (ROOT/'.env').read_text().splitlines():
  if '=' in line:
   key,value=line.split('=',1)
   if key.startswith('DEMO_') and key.endswith('ACCESS_CODE'):env[key]=value
version=subprocess.run([flutter,'--no-version-check','--version','--machine'],env=env,check=True,capture_output=True,text=True).stdout
if json.loads(version[version.index('{'):]).get('frameworkVersion')!='3.41.7':raise SystemExit('Select Flutter 3.41.7 for reproducible tests; the shared installation was not changed.')
raise SystemExit(subprocess.run([flutter,'--no-version-check',*(command or ['doctor','-v'])],cwd=ROOT/'mobile_app',env=env).returncode)
