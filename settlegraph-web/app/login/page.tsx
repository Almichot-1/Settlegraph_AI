'use client';
import { FormEvent,useState } from 'react';
import { useRouter } from 'next/navigation';
import { Activity, ArrowRight, LockKeyhole, ShieldCheck } from 'lucide-react';
export default function Login(){
 const [display_name,setName]=useState('Demo Owner'),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 const router=useRouter();
 async function submit(event:FormEvent){event.preventDefault();setBusy(true);setError('');try{
  const response=await fetch('/api/session',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({display_name})});
  const body=await response.json();if(!response.ok)throw new Error(body.error?.message??'Sign-in failed');router.replace('/dashboard');router.refresh();
 }catch(e){setError(e instanceof Error?e.message:'Cannot sign in');}finally{setBusy(false);}}
 return <main className="login-bg"><div className="login-glow"/><div className="login-frame"><div className="login-story"><div className="login-brand"><div className="brand-mark"><Activity size={23}/></div><strong>settlegraph<span>.ai</span></strong></div><div className="login-big"><div className="eyebrow">FINANCIAL CLARITY, ENGINEERED.</div><h1>Every payment has a story.<br/><em>Find the truth.</em></h1><p>One workspace to reconcile payment activity, identify inconsistencies, and investigate what the evidence actually shows.</p></div><div className="login-points"><span><ShieldCheck size={17}/>Evidence-backed matching</span><span><LockKeyhole size={17}/>No real bank connections</span></div></div><div className="login-card"><span className="demo-tag">HACKATHON DEMO</span><h2>Welcome back</h2><p>Enter the simulated merchant workspace.</p><form onSubmit={submit}><label htmlFor="display">Display name</label><input id="display" value={display_name} onChange={e=>setName(e.target.value)} maxLength={80} required/><button type="submit" className="button button-primary" disabled={busy}>{busy?'Signing in…':'Enter workspace'}<ArrowRight size={17}/></button>{error&&<div className="form-error" role="alert">{error}</div>}</form><div className="login-disclaimer"><ShieldCheck size={17}/> This environment uses synthetic financial records. No real funds or bank accounts are connected.</div></div></div></main>;
}
