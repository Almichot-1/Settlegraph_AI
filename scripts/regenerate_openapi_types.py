#!/usr/bin/env python3
"""Regenerate SettleGraph TypeScript schema aliases and API operation manifest.

Requires: pip install pyyaml
Run from anywhere: python scripts/regenerate_openapi_types.py
Keep the contract synchronized with the backend owner.
"""
import json
import hashlib
import sys
from pathlib import Path
import yaml
ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / 'contracts/settlegraph-public.openapi.yaml'
OUTPUT = ROOT / 'settlegraph-web/src/generated'

def expression(node):
    if not isinstance(node, dict):
        return 'unknown'
    if '$ref' in node:
        result = node['$ref'].split('/')[-1]
    elif 'enum' in node:
        result = ' | '.join(json.dumps(i) for i in node['enum'])
    elif node.get('type') == 'array':
        result = f'Array<{expression(node.get("items", {}))}>'
    elif node.get('type') == 'string':
        result = 'string'
    elif node.get('type') in ('number', 'integer'):
        result = 'number'
    elif node.get('type') == 'boolean':
        result = 'boolean'
    elif node.get('type') == 'object' or 'properties' in node:
        fields = node.get('properties', {})
        required = set(node.get('required', []))
        if fields:
            result = '{\n' + '\n'.join(f'  {json.dumps(k)}{"" if k in required else "?"}: {expression(v)};' for k, v in fields.items()) + '\n}'
        else:
            extra = node.get('additionalProperties', False)
            result = ('Record<string, unknown>' if extra is True else f'Record<string, {expression(extra)}>' if isinstance(extra, dict) else 'Record<string, never>')
    else:
        result = 'unknown'
    return f'({result}) | null' if node.get('nullable') else result

def write(path, content):
    if '--check' in sys.argv:
        if not path.exists() or path.read_text() != content:
            raise SystemExit('Generated contract drift: ' + str(path.relative_to(ROOT)))
    else:
        path.write_text(content)

def main():
    spec = yaml.safe_load(SOURCE.read_text())
    declarations = ['/** GENERATED FROM ../../contracts/settlegraph-public.openapi.yaml. DO NOT EDIT BY HAND. */',
                    '/** The authoritative schema is OpenAPI 3.0.3, version 2.0.0-draft. */']
    for name, schema in spec['components']['schemas'].items():
        declarations.append(f'export type {name} = {expression(schema)};\n')
    write(OUTPUT / 'types.ts', '\n'.join(declarations))
    operations = []
    for url, verbs in spec['paths'].items():
        for verb, op in verbs.items():
            if verb.upper() not in ('GET', 'POST', 'PATCH', 'PUT', 'DELETE'):
                continue
            success_code, response = sorted((int(k), v) for k, v in op.get('responses', {}).items() if str(k).isdigit() and 200 <= int(k) < 300)[0]
            response_type = response.get('content', {}).get('application/json', {}).get('schema', {}).get('$ref', '').split('/')[-1] or 'unknown'
            request_type = op.get('requestBody', {}).get('content', {}).get('application/json', {}).get('schema', {}).get('$ref', '').split('/')[-1] or None
            operations.append(dict(operationId=op['operationId'], method=verb.upper(), path=url, successStatus=success_code,
                                   response=response_type, request=request_type, priority=op.get('x-implementation-priority')))
    write(OUTPUT / 'operation-manifest.ts', '/** GENERATED — matches the public API contract. */\nexport const operationManifest = '+json.dumps(operations, indent=2)+' as const;\n')
    write(ROOT / 'docs/OPERATION_MANIFEST.json', json.dumps(operations, indent=2)+'\n')
    write(OUTPUT / 'contract-sha256.txt', hashlib.sha256(SOURCE.read_bytes()).hexdigest() + '\n')
    print(f'Generated {len(spec["components"]["schemas"])} TS schema aliases and {len(operations)} operations')

if __name__ == '__main__': main()
