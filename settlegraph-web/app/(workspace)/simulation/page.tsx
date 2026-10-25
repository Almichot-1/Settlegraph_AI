'use client';
import {useCallback,useEffect,useState} from 'react';
import {ArrowRight,FlaskConical,Play,RefreshCw,StepForward} from 'lucide-react';
import {api} from '@/lib/api';
import type {ScenarioDefinition,ScenarioRun} from '@/generated/types';
import {Badge,Failure,Loading,Notice,Panel,SectionHeading} from '@/components/ui';
export default function Simulation(){
 const [catalog,setCatalog]=useState<ScenarioDefinition[]>([]),[selected,setSelected]=useState<ScenarioDefinition['key']>('delayed_settlement'),[run,setRun]=useState<ScenarioRun|null>(null),[busy,setBusy]=useState(true),[working,setWorking]=useState(false),[error,setError]=useState<unknown>(null),[message,setMessage]=useState('');
 const load=useCallback(async()=>{setBusy(true);try{const result=await api.listDemoScenarios();setCatalog(result.items);setError(null);}catch(e){setError(e);}finally{setBusy(false);}},[]);
 useEffect(()=>{void load();},[load]);
 async function start(){setWorking(true);setMessage('');try{const r=await api.createScenarioRun(selected,selected==='delayed_settlement'?102:101);setRun(r);setMessage('Simulated bank scenario started. Advance one step at a time.');}catch(e){setMessage(e instanceof Error?e.message:'Unable to start scenario');}finally{setWorking(false);}}
 async function advance(){if(!run)return;setWorking(true);setMessage('');try{const r=await api.advanceScenarioRun(run.id,run.step);setRun(r);setMessage('Step accepted. Financial observations may require background processing before appearing in other pages.');}catch(e){setMessage(e instanceof Error?e.message:'Failed to advance scenario');}finally{setWorking(false);}}
 async function refresh(){if(!run)return;try{setRun(await api.getScenarioRun(run.id));}catch(e){setMessage(e instanceof Error?e.message:'Unable to refresh');}}
 return <><SectionHeading title="BankSim control room" description="Run deterministic financial scenarios without connecting to actual banks." action={<span className="pill-label"><FlaskConical size={15}/> SYNTHETIC BANK NETWORK</span>}/>
 <Notice tone="warn">BankSim is a testing environment. These transfers and balances are simulated; they do not represent transactions at real Ethiopian banks.</Notice>
 <div className="dashboard-columns"><Panel title="Scenario library" subtitle="Choose a fault-injection scenario">
 {error ? <Failure error={error} onRetry={load}/> : null} {busy?<Loading/>:<div className="scenario-list">{catalog.map(x=><button className={`scenario-option ${selected===x.key?'chosen':''}`} key={x.key} onClick={()=>setSelected(x.key)} disabled={Boolean(run&&run.status!=='COMPLETED')}><div className="scenario-radio"/><div><strong>{x.title}</strong><p>{x.description}</p><small>{x.steps} deterministic steps</small></div></button>)}</div>}
 <button className="button button-primary button-full" disabled={working||Boolean(run&&run.status!=='COMPLETED')} onClick={start}><Play size={16}/>{working?'Working…':'Start selected scenario'}</button>
 </Panel><Panel title="Execution timeline" subtitle="Control and observe asynchronous financial events">
 {run?<><div className="run-summary"><div><span>Scenario</span><strong>{run.scenario_key.replaceAll('_',' ')}</strong></div><Badge value={run.status}/></div><div className="progress-track"><div style={{width:`${(run.step/run.total_steps)*100}%`}}/></div><div className="run-step">Step {run.step} of {run.total_steps}</div><div className="run-id">Run ID: {run.id}</div><div className="run-buttons"><button className="button button-primary" onClick={advance} disabled={working||run.status==='COMPLETED'}><StepForward size={16}/> Advance step</button><button className="button button-ghost" onClick={refresh}><RefreshCw size={16}/>Refresh</button></div><div className="sim-links"><a href="/transactions">View payment observations <ArrowRight size={15}/></a><a href="/cases">View resulting cases <ArrowRight size={15}/></a></div></>:<div className="run-placeholder"><FlaskConical size={31}/><strong>No scenario running</strong><p>Start a scenario on the left to inspect transitions, discrepancies and recovery.</p></div>}
 </Panel></div>
 {message&&<div className="inline-message" role="status">{message}</div>}
 <Panel title="What judges should observe" subtitle="A deterministic end-to-end reliability demonstration"><div className="demo-steps"><div><span>01</span><strong>Payment accepted</strong><p>BankSim emits a simulated payment record.</p></div><div><span>02</span><strong>Missing or stale evidence</strong><p>SettleGraph detects a mismatch or missing source window.</p></div><div><span>03</span><strong>Evidence-backed investigation</strong><p>AI explains possibilities; the verification engine prevents unsupported conclusions.</p></div><div><span>04</span><strong>Recovery and audit</strong><p>The delayed event arrives. The case updates while earlier findings remain visible.</p></div></div></Panel>
 </>;
}
