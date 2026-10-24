#!/usr/bin/env python3
"""Derive a dependency-checked, machine-readable task graph from PM playbook.
This is design data, not evidence of implementation.
"""
from pathlib import Path
import re,json
ROOT=Path(__file__).resolve().parents[1]
s=(ROOT/'PROJECT_MANAGER_DELIVERY_PLAYBOOK.md').read_text()
heads=list(re.finditer(r'^\*\*(SG-\d{3}) — (.*?)\*\* · Owner: (.*?) · Depends: (.*?) · (M\d)\s*$',s,re.M))
tasks=[]
for i,m in enumerate(heads):
    body=s[m.end(): (heads[i+1].start() if i+1<len(heads) else s.index('### Backlog discipline',m.end()))]
    lines=[x[2:].strip() for x in body.splitlines() if x.startswith('- ')]
    dep=m.group(4)
    deps=[] if dep=='none' else re.findall(r'SG-\d{3}',dep)
    criteria=[x.removeprefix('**Accept:** ').strip() for x in lines if x.startswith('**Accept:**')]
    work=[x for x in lines if not x.startswith('**Accept:**')]
    tasks.append({'id':m.group(1),'title':m.group(2),'owner':m.group(3),'priority':m.group(5),'dependencies':deps,
                  'description':work,'acceptance':criteria,'status':'NOT_STARTED','evidence':{'commit':None,'tests':[],'runtime_verified':False}})
ids={x['id'] for x in tasks}
assert len(tasks)==22, f'Expected 22 tickets, got {len(tasks)}'
assert len(ids)==len(tasks), 'Duplicate ticket identifiers'
assert all(set(t['dependencies']).issubset(ids) for t in tasks), 'Unknown dependency'
assert all(t['acceptance'] for t in tasks), 'Missing acceptance'
# Validate acyclic graph
visiting=set();visited=set()
byid={t['id']:t for t in tasks}
def dfs(i):
    if i in visiting:raise AssertionError('Dependency cycle '+i)
    if i in visited:return
    visiting.add(i)
    for dep in byid[i]['dependencies']:dfs(dep)
    visiting.remove(i);visited.add(i)
for t in tasks:dfs(t['id'])
output={'project':'SettleGraph AI','version':'2.1','status':'SPECIFICATION_ONLY','actual_tests_executed':False,
        'contract':'contracts/settlegraph-public.openapi.yaml','tickets':tasks}
p=ROOT/'AGENT_BACKLOG.json';p.write_text(json.dumps(output,indent=2)+'\n')
print(f'Wrote {p.name}: {len(tasks)} tickets, dependencies acyclic, acceptance present')
