#!/usr/bin/env python3
"""Prefetch the checksum-pinned Gradle wrapper archive into the project cache."""
import hashlib
from pathlib import Path
import subprocess
import sys
ROOT=Path(__file__).resolve().parents[1]
settings=dict(line.split('=',1) for line in (ROOT/'mobile_app/android/gradle/wrapper/gradle-wrapper.properties').read_text().splitlines() if '=' in line)
url=settings['distributionUrl'].replace('\\:',':')
checksum=settings['distributionSha256Sum']
value=int.from_bytes(hashlib.md5(url.encode(),usedforsecurity=False).digest(),'big')
alphabet='0123456789abcdefghijklmnopqrstuvwxyz';key=''
while value:value,digit=divmod(value,36);key=alphabet[digit]+key
name=url.rsplit('/',1)[1]
destination=ROOT/'.tools/gradle/wrapper/dists'/name.removesuffix('.zip')/key/name
subprocess.run([sys.executable,str(ROOT/'scripts/download_tool.py'),url,str(destination),'--sha256',checksum,'--workers','32'],check=True)
print('Gradle wrapper archive ready; the wrapper will verify and extract it on first build.')
