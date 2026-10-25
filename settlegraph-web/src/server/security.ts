import { operationManifest } from '@/generated/operation-manifest';

export function publicOrigin(req:Request):string|null {
  if(process.env.SETTLEGRAPH_WEB_ORIGIN) {
    try{return new URL(process.env.SETTLEGRAPH_WEB_ORIGIN).origin;}catch{return null;}
  }
  if(process.env.NODE_ENV==='production')return null;
  try {
    const value=new URL(new URL(req.url).protocol+'//'+req.headers.get('host'));
    return ['localhost','127.0.0.1','[::1]'].includes(value.hostname)?value.origin:null;
  } catch{return null;}
}

export function sameOrigin(req: Request, origin: string|null): boolean {
  if(!origin)return false;
  const supplied = req.headers.get('origin');
  if (supplied) return supplied === origin;
  const referer = req.headers.get('referer');
  if (referer) { try { return new URL(referer).origin === origin; } catch { return false; } }
  const site = req.headers.get('sec-fetch-site');
  return !site || site === 'same-origin' || site === 'none';
}

export function allowedOperation(method: string, path: string): boolean {
  return operationManifest.some(op => {
    if (op.operationId === 'createDemoSession' || op.operationId === 'acceptBankSimEvent' || op.method !== method) return false;
    const template = op.path.split('/');
    const segments = path.split('/');
    return template.length === segments.length && template.every((part, i) => part.startsWith('{') ? /^[A-Za-z0-9_-]+$/.test(segments[i]) : part === segments[i]);
  });
}

export async function limitedText(req: Request, maximum = 1048576): Promise<string> {
  const length = req.headers.get('content-length');
  if (length && (!/^\d+$/.test(length) || Number(length) > maximum)) throw new Error('PAYLOAD_TOO_LARGE');
  const reader = req.body?.getReader();
  if (!reader) return '';
  const chunks: Uint8Array[] = [];
  let bytes = 0;
  try {
    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      bytes += value.byteLength;
      if (bytes > maximum) { await reader.cancel(); throw new Error('PAYLOAD_TOO_LARGE'); }
      chunks.push(value);
    }
  } finally { reader.releaseLock(); }
  const combined = new Uint8Array(bytes);
  let offset = 0;
  for (const chunk of chunks) { combined.set(chunk, offset); offset += chunk.length; }
  return new TextDecoder('utf-8', {fatal: true}).decode(combined);
}
