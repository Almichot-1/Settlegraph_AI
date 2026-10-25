#!/usr/bin/env python3
"""Run every Go integration test against its own migrated/seeded database.

Only the disposable container created here is removed; demo data is untouched.
"""
import os
from pathlib import Path
import re
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]
os.chdir(ROOT)
name = 'settlegraph-tests-' + uuid.uuid4().hex[:12]

def run(*args, capture=False, **kwargs):
    return subprocess.run(args, check=True, text=True, capture_output=capture, **kwargs)

def sql(*args):
    return run('docker', 'exec', name, 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres', *args, capture=True)

try:
    run('docker', 'run', '-d', '--name', name, '-e', 'POSTGRES_HOST_AUTH_METHOD=trust',
        '-p', '127.0.0.1::5432', '-v', str(ROOT / 'db') + ':/migrations:ro', 'postgres:17.6-alpine', capture=True)
    for _ in range(60):
        # The image's temporary bootstrap server accepts Unix sockets before
        # initialization ends. TCP readiness selects the final server.
        if subprocess.run(['docker', 'exec', name, 'pg_isready', '-h', '127.0.0.1', '-U', 'postgres'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
            break
        time.sleep(.5)
    else:
        raise RuntimeError('Isolated PostgreSQL did not become ready')
    port = run('docker', 'port', name, '5432', capture=True).stdout.strip().rsplit(':', 1)[1]
    for path in sorted((ROOT / 'internal').rglob('*_test.go')):
        source = path.read_text()
        if not source.startswith('//go:build integration'):
            continue
        bank = path.parent.name == 'banksim'
        domain = 'banksim' if bank else 'settlegraph'
        for test in re.findall(r'^func (Test\w+)\(t \*testing.T\)', source, re.M):
            database = 'test_' + uuid.uuid4().hex[:16]
            sql('-d', 'postgres', '-c', 'CREATE DATABASE ' + database)
            for _ in range(2):
                sql('-d', database, '-f', '/migrations/apply_' + domain + '.sql')
                sql('-d', database, '-f', '/migrations/seed_' + domain + '.sql')
            env = os.environ.copy()
            env['BANKSIM_TEST_DATABASE_URL' if bank else 'PRODUCT_TEST_DATABASE_URL'] = f'postgres://postgres@127.0.0.1:{port}/{database}?sslmode=disable'
            print('Running', test, 'with a fresh isolated database', flush=True)
            result = run('go', 'test', '-tags', 'integration', '-count=1', '-timeout', '120s',
                         './' + str(path.parent.relative_to(ROOT)), '-run', '^' + test + '$', '-v', env=env, capture=True)
            print(result.stdout, end='', flush=True)
            if '--- SKIP:' in result.stdout:
                raise RuntimeError('Integration test skipped: ' + test)
    print('PASS: isolated Go integration tests; migrations/seeds applied twice per database')
except subprocess.CalledProcessError as error:
    if error.stdout:
        print(error.stdout)
    if error.stderr:
        print(error.stderr)
    raise SystemExit(error.returncode)
finally:
    subprocess.run(['docker', 'rm', '-f', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
