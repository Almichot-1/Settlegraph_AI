'use client';
import {useCallback,useEffect,useState} from 'react';
import {RefreshCw} from 'lucide-react';
import {api} from '@/lib/api';
import type {AuditEvent} from '@/generated/types';
import {shortDate} from '@/lib/format';
import {Empty,Failure,Loading,Panel,SectionHeading} from '@/components/ui';
export default function Audit(){
 const [cursor,setCursor]=useState<string|null>(null);
 const [items,setItems]=useState<AuditEvent[]>([]),[busy,setBusy]=useState(true),[error,setError]=useState<unknown>(null);
 const load=useCallback(async(next?:string)=>{setBusy(true);try{const r=await api.listAuditEvents({limit:100,cursor:next});setItems(old=>next?[...old,...r.items]:r.items);setCursor(r.next_cursor);setError(null);}catch(e){setError(e);}finally{setBusy(false);}},[]);useEffect(()=>{void load();},[load]);
 return <><SectionHeading title="Audit trail" description="Traceable actions across simulated payments and investigations." action={<button className="button button-ghost" onClick={()=>void load()}><RefreshCw size={16}/>Refresh</button>}/><Panel title="Recorded events">{error ? <Failure error={error} onRetry={()=>void load()}/> : null} {busy&&!items.length?<Loading/>:items.length?<div className="table-scroll"><table><thead><tr><th>Timestamp</th><th>Actor</th><th>Action</th><th>Resource</th><th>ID</th></tr></thead><tbody>{items.map(a=><tr key={a.id}><td>{shortDate(a.occurred_at)}</td><td>{a.actor}</td><td>{a.action.replaceAll('_',' ')}</td><td>{a.resource_type}</td><td className="mono">{a.resource_id.slice(0,16)}…</td></tr>)}</tbody></table></div>:<Empty text="Actions will appear after you run a simulation or make a case decision."/>}{cursor&&<button className="button button-ghost" disabled={busy} onClick={()=>void load(cursor)}>Load more events</button>}</Panel></>;
}
