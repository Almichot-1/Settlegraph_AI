#!/usr/bin/env python3
"""Copy canonical synthetic fixtures, or verify the bundled copies in CI."""
from pathlib import Path
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('--check', action='store_true')
args = parser.parse_args()
mobile = Path(__file__).resolve().parents[1]
source = mobile.parent / 'fixtures'
target = mobile / 'assets' / 'fixtures'
errors = []
for fixture in source.glob('*.json'):
    destination = target / fixture.name
    if args.check:
        if not destination.exists() or destination.read_bytes() != fixture.read_bytes():
            errors.append(fixture.name)
    else:
        target.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(fixture.read_bytes())
if errors:
    raise SystemExit('Bundled fixtures differ: ' + ', '.join(errors))
print('PASS: mobile fixture copies match canonical fixtures' if args.check else 'Copied canonical synthetic fixtures')
