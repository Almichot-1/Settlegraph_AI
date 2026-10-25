#!/usr/bin/env python3
"""Start the integrated local demo. Stop retains all database volumes."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('action', choices=['up', 'down', 'status', 'logs'])
args = parser.parse_args()
os.chdir(ROOT)
if not shutil.which('docker'):
    raise SystemExit('Docker with Compose is required. Install/start Docker, then rerun make local-up.')

def run(*command):
    subprocess.run(command, check=True)

def compose(*command):
    run('docker', 'compose', '-f', 'compose.yaml', '-f', 'compose.web.yaml', *command)

if args.action == 'up':
    run(sys.executable, 'validation/init_demo_env.py')
    compose('up', '-d', '--build', '--wait', 'product-db', 'banksim-db', 'api', 'worker', 'banksim')
    run('make', 'migrate')
    run('make', 'seed')
    compose('up', '-d', '--build', '--wait', 'web')
    print('Web: http://localhost:' + os.environ.get('WEB_PORT', '3000'))
    print('API: http://localhost:' + os.environ.get('API_PORT', '8080'))
elif args.action == 'down':
    compose('down')
elif args.action == 'status':
    compose('ps')
else:
    compose('logs', '--tail', '80')
