import { ApiRequestError } from './client';
import { makeKey } from './format';

export type Attempt = {key:string;body:unknown;version?:number;created:number};
const prefix='settlegraph.pending.v1:';
const maxAge=60*60*1000;
function storage():Storage|undefined {try{return typeof window==='undefined'?undefined:window.sessionStorage;}catch{return undefined;}}
export function pendingAttempt(scope:string):Attempt|null {
  try {
    const store=storage();const raw=store?.getItem(prefix+scope);if(!raw)return null;
    const value=JSON.parse(raw) as Attempt;
    if(typeof value.key!=='string'||!Number.isFinite(value.created)||Date.now()-value.created>maxAge){store?.removeItem(prefix+scope);return null;}
    return value;
  } catch {return null;}
}
export function clearAttempts(){const store=storage();if(store)for(const key of Object.keys(store))if(key.startsWith(prefix))store.removeItem(key);}
export async function runMutation<T>(scope:string,body:unknown,version:number|undefined,execute:(attempt:Attempt)=>Promise<T>):Promise<T> {
  const attempt=pendingAttempt(scope)??{key:makeKey(),body,version,created:Date.now()};
  const store=storage();
  // Persist before sending. If storage is unavailable, stop before creating an
  // action whose retry identity would be lost on navigation.
  if(!store)throw new Error('Browser storage is unavailable. Enable session storage before submitting.');
  store.setItem(prefix+scope,JSON.stringify(attempt));
  try {
    const result=await execute(attempt);store.removeItem(prefix+scope);return result;
  } catch(error) {
    if(error instanceof ApiRequestError&&error.status<500)store.removeItem(prefix+scope);
    throw error;
  }
}
