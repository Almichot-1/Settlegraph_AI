#!/usr/bin/env python3
"""Create local-only demo credentials without echoing them or replacing existing values."""
from pathlib import Path
import secrets

path = Path('.env')
existing = path.read_text() if path.exists() else ''
keys = {line.split('=', 1)[0] for line in existing.splitlines() if '=' in line}
with path.open('a') as out:
    for name, length in [('DEMO_ACCESS_CODE', 24), ('DEMO_JWT_SECRET', 48),
                         ('BANKSIM_ADMIN_TOKEN', 48), ('BANKSIM_PROVIDER_TOKEN', 48),
                         ('BANKSIM_EVALUATOR_TOKEN', 48), ('BANKSIM_WEBHOOK_SECRET', 48)]:
        if name not in keys:
            out.write(f'{name}={secrets.token_urlsafe(length)}\n')
path.chmod(0o600)
