#!/usr/bin/env python3
"""Download an official tool archive with bounded parallel ranges and SHA-256.

Part files are retained for resuming interrupted setup, never treated as valid
until the complete archive checksum matches the supplied published checksum.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
from pathlib import Path
import time
import urllib.request

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('url');p.add_argument('destination',type=Path);checks=p.add_mutually_exclusive_group(required=True)
checks.add_argument('--sha256');checks.add_argument('--sha1')
p.add_argument('--workers',type=int,default=12)
a=p.parse_args()
def digest(path):
 h=hashlib.new('sha256' if a.sha256 else 'sha1')
 with path.open('rb') as source:
  while data:=source.read(1024*1024):h.update(data)
 return h.hexdigest()

a.destination.parent.mkdir(parents=True,exist_ok=True)
if a.destination.exists() and digest(a.destination)==(a.sha256 or a.sha1):
 print('Verified cached archive:',a.destination);raise SystemExit()
for attempt in range(4):
 try:
  with urllib.request.urlopen(urllib.request.Request(a.url,method='HEAD'),timeout=30) as r:
   total=int(r.headers['Content-Length']);resolved_url=r.url
  break
 except Exception:
  if attempt==3:raise
  time.sleep(attempt+1)
chunk=1024*1024
parts=a.destination.parent/(a.destination.name+'.parts');parts.mkdir(exist_ok=True)
def download(index):
 start=index*chunk;end=min(total,start+chunk)-1
 target=parts/str(index)
 if target.exists() and target.stat().st_size==end-start+1:return
 for attempt in range(4):
  try:
   req=urllib.request.Request(resolved_url,headers={'Range':f'bytes={start}-{end}','Accept-Encoding':'identity'})
   with urllib.request.urlopen(req,timeout=180) as r:
    if r.status!=206:raise RuntimeError('Server did not honor range request')
    data=r.read(end-start+2)
   if len(data)!=end-start+1:raise RuntimeError('Incomplete range')
   target.write_bytes(data);return
  except Exception:
   if attempt==3:raise
   time.sleep(attempt+1)
count=(total+chunk-1)//chunk
with ThreadPoolExecutor(max_workers=max(1,min(a.workers,32))) as pool:
 for complete,future in enumerate(as_completed([pool.submit(download,i) for i in range(count)]),1):
  future.result()
  if complete%20==0 or complete==count:print(f'Downloaded {complete}/{count} parts',flush=True)
temporary=a.destination.with_suffix(a.destination.suffix+'.assembling')
with temporary.open('wb') as out:
 for i in range(count):out.write((parts/str(i)).read_bytes())
if digest(temporary)!=(a.sha256 or a.sha1):raise SystemExit('Checksum mismatch; archive was not installed')
temporary.replace(a.destination)
print('Verified archive:',a.destination)
