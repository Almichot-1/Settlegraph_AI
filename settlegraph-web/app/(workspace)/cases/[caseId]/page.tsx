'use client';
import Link from 'next/link';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useParams } from 'next/navigation';
import { ArrowLeft, BrainCircuit, RefreshCw, Send, ShieldCheck, UserCheck } from 'lucide-react';
import { api } from '@/lib/api';
import { useWorkspaceSession, activeRole, workspaceScope } from '@/lib/session';
import { runMutation, pendingAttempt } from '@/lib/mutations';
import { allPages, ApiRequestError } from '@/lib/client';
import type { Case, Evidence, CaseEvent, Investigation, ReviewDecisionRequest, Member } from '@/generated/types';
import { formatMoney, shortDate } from '@/lib/format';
import { Badge, Empty, Failure, Loading, Notice, Panel } from '@/components/ui';

const MAX_POLL_ATTEMPTS = 15;
const POLL_INTERVAL_MS = 2000;

export default function CaseDetail() {
  const session=useWorkspaceSession();
  const role=activeRole(session);
  const canAssign=['OWNER','ANALYST'].includes(role??'');
  const canReview=['OWNER','REVIEWER'].includes(role??'');
  const scope=workspaceScope(session)+':case:';
  const [members,setMembers]=useState<Member[]>([]);
  const [ackRequired,setAckRequired]=useState(false);
  const [uncertain,setUncertain]=useState(false);
  const loadGeneration=useRef(0);
  const { caseId } = useParams<{ caseId: string }>();
  const [record, setRecord] = useState<Case | null>(null);
  const [evidence, setEvidence] = useState<Evidence[]>([]);
  const [events, setEvents] = useState<CaseEvent[]>([]);
  const [investigations, setInvestigations] = useState<Investigation[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(true);
  const [actionBusy, setActionBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [reason, setReason] = useState('Verified the latest available evidence records.');
  const [decision, setDecision] = useState<ReviewDecisionRequest['decision']>('NEEDS_MORE_EVIDENCE');
  const [selectedAssignee, setSelectedAssignee] = useState<string>('');
  // Stable idempotency key for decision mutations across retries

  const [pollingAiId, setPollingAiId] = useState<string | null>(null);
  const aiPollTimerRef = useRef<NodeJS.Timeout | null>(null);
  const aiPollCountRef = useRef(0);

  const load = useCallback(async () => {
    const generation=++loadGeneration.current;
    setBusy(true);
    try {
      let c:Case|undefined;
      let e:Evidence[]=[];let ev:CaseEvent[]=[];let ai:Investigation[]=[];
      for(let attempt=0;attempt<3;attempt++) {
        const before=await api.getCase(caseId);
        [e,ev,ai]=await Promise.all([
          allPages(cursor=>api.listCaseEvidence(caseId,{limit:100,cursor})),
          allPages(cursor=>api.listCaseEvents(caseId,{limit:100,cursor})),
          allPages(cursor=>api.listInvestigations(caseId,{limit:100,cursor})),
        ]);
        const after=await api.getCase(caseId);
        if(before.version===after.version){c=after;break;}
      }
      if(!c)throw new Error('Evidence is changing. Refresh before reviewing.');
      if(generation!==loadGeneration.current)return;
      const directory=canAssign?await allPages(cursor=>api.listMembers({limit:100,cursor})):[];
      if(generation!==loadGeneration.current)return;
      setMembers(directory.filter(m=>m.role!=='VIEWER'));
      setRecord(c);
      setSelectedAssignee(c.assigned_to??'');
      setEvidence(e);setEvents(ev);
      setInvestigations(ai.sort((a,b)=>b.created_at.localeCompare(a.created_at)||b.id.localeCompare(a.id)));
      const pending=pendingAttempt(scope+caseId+':decision');
      setUncertain(Boolean(pending));
      if(pending){const body=pending.body as ReviewDecisionRequest;setReason(body.reason);setDecision(body.decision);}
      setError(null);
    } catch (e) {
      if(generation===loadGeneration.current)setError(e);
    } finally {
      if(generation===loadGeneration.current)setBusy(false);
    }
  }, [caseId,canAssign,scope]);

  useEffect(() => {
    setRecord(null);setAckRequired(false);setPollingAiId(null);
    void load();
    return () => {
      loadGeneration.current++;
      if (aiPollTimerRef.current) clearTimeout(aiPollTimerRef.current);
    };
  }, [load]);

  // Bounded polling for asynchronous HTTP 202 investigation results
  const pollInvestigation = useCallback(
    (invId: string) => {
      if (aiPollTimerRef.current) clearTimeout(aiPollTimerRef.current);
      aiPollCountRef.current = 0;
      setPollingAiId(invId);
      const generation=loadGeneration.current;

      const check = async () => {
        if(generation!==loadGeneration.current)return;
        aiPollCountRef.current += 1;
        try {
          const inv = await api.getInvestigation(caseId, invId);
          const aiList = await allPages(cursor=>api.listInvestigations(caseId,{limit:100,cursor}));
          if(generation!==loadGeneration.current)return;
          setInvestigations(aiList.sort((a,b)=>b.created_at.localeCompare(a.created_at)));

          if (inv.status === 'COMPLETED') {
            await load();
            setMessage('AI investigation completed with evidence citations.');
            setPollingAiId(null);
            return;
          }
          if (inv.status === 'REJECTED' || inv.status === 'FAILED') {
            setMessage(`AI investigation ended with status: ${inv.status}.`);
            setPollingAiId(null);
            return;
          }

          if (aiPollCountRef.current < MAX_POLL_ATTEMPTS) {
            aiPollTimerRef.current = setTimeout(check, POLL_INTERVAL_MS);
          } else {
            setMessage('AI investigation is still processing in background. Refresh to view results.');
            setPollingAiId(null);
          }
        } catch (err) {
          if(generation!==loadGeneration.current)return;
          setMessage(err instanceof Error ? err.message : 'Error polling investigation');
          setPollingAiId(null);
        }
      };

      aiPollTimerRef.current = setTimeout(check, POLL_INTERVAL_MS);
    },
    [caseId,load]
  );

  async function investigate() {
    setActionBusy(true);
    setMessage('');
    try {
      const ai = await runMutation(scope+caseId+':investigate',{mode:'EVIDENCE_ANALYSIS'},undefined,m=>api.createInvestigation(caseId,m.body as {mode:'EVIDENCE_ANALYSIS'},m.key));
      await load();

      if (ai.status === 'COMPLETED') {
        setMessage('AI investigation ready.');
      } else {
        setMessage('Investigation initiated (HTTP 202). Polling for evidence-gated results…');
        pollInvestigation(ai.id);
      }
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Investigation request failed');
    } finally {
      setActionBusy(false);
    }
  }

  async function assign() {
    if (!record || !selectedAssignee) return;
    setActionBusy(true);
    setMessage('');
    try {
      const updated = await runMutation(scope+caseId+':assign',{assignee_id:selectedAssignee},record.version,m=>api.assignCase(caseId,m.body as {assignee_id:string},m.version!,m.key));
      setRecord(updated);
      const memberName = members.find((m) => m.id === selectedAssignee)?.display_name ?? selectedAssignee;
      setMessage(`Case successfully reassigned to ${memberName}. Version updated to v${updated.version}.`);
      await load();
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 412) {
        setMessage('Case version conflict (HTTP 412): Someone updated this case. Reloading latest case…');
        await load();
      } else {
        setMessage(e instanceof Error ? e.message : 'Could not assign case');
      }
    } finally {
      setActionBusy(false);
    }
  }

  async function review() {
    if (!record) return;
    const cleanReason = reason.trim();
    if (cleanReason.length < 5) {
      setMessage('Reason must contain at least 5 characters.');
      return;
    }

    setActionBusy(true);
    setMessage('');
    try {
      // Submitting review decision requiring explicit reason (>= 5 chars), If-Match: "<Case.version>", and stable Idempotency-Key
      const c=await runMutation(scope+caseId+':decision',{decision,reason:cleanReason},record.version,
        m=>api.recordCaseDecision(caseId,m.body as ReviewDecisionRequest,m.version!,m.key));
      setRecord(c);
      setMessage(`Decision "${decision}" recorded successfully with audit history.`);
      // Rotate stable idempotency key after successful mutation
      setUncertain(false);
      await load();
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 412) {
        // HTTP 412 stale version conflict: notify user and refresh latest case state
        setMessage(
          'Case version conflict (HTTP 412): Someone updated this case. Latest version reloaded—please re-evaluate and re-submit your decision.'
        );
        setAckRequired(true);setUncertain(false);
        await load();
      } else {
        setUncertain(Boolean(pendingAttempt(scope+caseId+':decision')));
        setMessage(e instanceof Error ? e.message : 'Could not record decision');
      }
    } finally {
      setActionBusy(false);
    }
  }

  if (busy && !record) return <Loading />;
  if (error && !record) return <Failure error={error} onRetry={load} />;
  if (!record) return <Empty />;

  // Ensure CaseEvents timeline is strictly chronological
  const chronologicalEvents = [...events].sort(
    (a, b) => new Date(a.occurred_at).getTime() - new Date(b.occurred_at).getTime() || a.version - b.version
  );

  return (
    <>
      <div className="case-detail-top">
        <Link href="/cases" className="back-link">
          <ArrowLeft size={16} /> Investigation inbox
        </Link>
        <button className="button button-ghost" onClick={load} disabled={busy || Boolean(pollingAiId)}>
          <RefreshCw size={16} className={busy || pollingAiId ? 'spin' : ''} />
          Refresh
        </button>
      </div>

      <div className="section-heading">
        <div>
          <div className="eyebrow">CASE #{record.id.slice(0, 8)}</div>
          <h1>{record.title}</h1>
          <p>{record.summary}</p>
        </div>
        <div className="detail-status">
          <Badge value={record.status} />
          <Badge value={record.severity} />
          <Badge value={record.kind} />
        </div>
      </div>

      <Notice>
        Cases summarize the evidence available in the workspace. An AI explanation is a hypothesis, not a bank confirmation. Only human reviewers have decision authority; AI cannot close or resolve cases.
      </Notice>

      {error&&<Failure error={error} onRetry={load}/>}
      <div className="case-detail-grid">
        <div className="vertical-stack">
          {/* Supporting evidence panel */}
          <Panel title="Supporting evidence" subtitle={`${evidence.length} source observations`}>
            <div className="evidence-list">
              {evidence.length ? (
                evidence.map((e) => (
                  <div className="evidence-card" id={`evidence-${e.id}`} key={e.id}>
                    <div className="evidence-head">
                      <Badge value={e.record_type} />
                      <span>{shortDate(e.occurred_at)}</span>
                    </div>
                    <strong className="mono">{e.reference}</strong>
                    <div className="evidence-foot">
                      <span>{e.provider_id ?? 'No provider'}</span>
                      <strong>{formatMoney(e.amount)}</strong>
                    </div>
                  </div>
                ))
              ) : (
                <Empty text="No evidence records attached to this case." />
              )}
            </div>
          </Panel>

          {/* Review history timeline */}
          <Panel title="Review history" subtitle="Append-only chronological events">
            <div className="timeline">
              {chronologicalEvents.map((e) => (
                <div className="timeline-item" key={e.id}>
                  <div className="timeline-dot" />
                  <div>
                    <strong>{e.type.replaceAll('_', ' ')}</strong>
                    <p>{e.message}</p>
                    <small>
                      {shortDate(e.occurred_at)} · v{e.version} · {e.actor}
                    </small>
                  </div>
                </div>
              ))}
              {!chronologicalEvents.length && <Empty text="No review events recorded." />}
            </div>
          </Panel>
        </div>

        <div className="vertical-stack">
          {/* AI investigation panel */}
          <Panel
            title="AI investigation"
            subtitle="Evidence-gated explanation"
            action={<BrainCircuit size={19} className="subtle-icon" />}
          >
            <button
              className="button button-primary button-full"
              onClick={investigate}
              disabled={!canAssign || actionBusy || busy || Boolean(error) || Boolean(pollingAiId)}
            >
              <BrainCircuit size={17} />
              {actionBusy ? 'Working…' : pollingAiId ? 'Polling analysis…' : 'Investigate evidence'}
            </button>

            {investigations.length ? (
              investigations.map((ai) => (
                <div className="ai-result" key={ai.id}>
                  <div className="ai-meta">
                    <Badge value={ai.status} />
                    <Badge value={ai.provider === 'OFFLINE_STUB' ? 'OFFLINE_STUB' : 'AMAZON BEDROCK'} />
                    <span>
                      {ai.provider === 'OFFLINE_STUB'
                        ? 'OFFLINE STUB — NOT LIVE BEDROCK'
                        : 'AMAZON BEDROCK'}
                    </span>
                  </div>

                  {ai.result ? (
                    <>
                      <h3>Summary</h3>
                      <p>{ai.result.summary}</p>

                      <h3>Possible explanations (Hypotheses)</h3>
                      {ai.result.hypotheses.map((h, i) => (
                        <div key={i} style={{ marginBottom: '8px' }}>
                          <p>• {h.explanation}</p>
                          {h.evidence_ids?.length > 0 && (
                            <div className="evidence-links">
                              {h.evidence_ids.map((id) => (
                                <a key={id} href={`#evidence-${id}`} className="mono">
                                  #evidence-{id.slice(0, 8)}…
                                </a>
                              ))}
                            </div>
                          )}
                        </div>
                      ))}

                      <h3>Supporting evidence IDs</h3>
                      <div className="evidence-links">
                        {ai.result.evidence_ids.map((id) => (
                          <a key={id} href={`#evidence-${id}`} className="mono">
                            #evidence-{id.slice(0, 8)}…
                          </a>
                        ))}
                      </div>

                      <h3>Recommended next actions</h3>
                      <ul>
                        {ai.result.recommended_actions.map((a, i) => (
                          <li key={i}>{a}</li>
                        ))}
                      </ul>

                      <div className="limitations">
                        <strong>Limitations:</strong> {ai.result.limitations.join(' ')}
                      </div>
                    </>
                  ) : (
                    <p>Awaiting investigation completion. Refresh for updates.</p>
                  )}
                </div>
              ))
            ) : (
              <p className="help-text">No AI investigation has been requested for this case yet.</p>
            )}
          </Panel>

          {/* Case assignment panel */}
          <Panel
            title="Case assignment"
            subtitle={`Current owner: ${members.find((m) => m.id === record.assigned_to)?.display_name ?? (record.assigned_to ?? 'Unassigned')}`}
            action={<UserCheck size={18} className="subtle-icon" />}
          >
            <label className="field-label" htmlFor="assignee-select">
              Assignee
            </label>
            <select
              id="assignee-select"
              className="full-field"
              value={selectedAssignee}
              disabled={!canAssign || Boolean(pendingAttempt(scope+caseId+':assign'))}
              onChange={(e) => setSelectedAssignee(e.target.value)}
            >
              <option value="">Choose a member</option>
              {members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.display_name} ({m.role})
                </option>
              ))}
            </select>

            <button
              onClick={assign}
              className="button button-ghost button-full"
              style={{ marginTop: '10px' }}
              disabled={!canAssign || actionBusy || busy || Boolean(error) || !selectedAssignee || selectedAssignee === record.assigned_to}
            >
              <UserCheck size={16} />
              {selectedAssignee === record.assigned_to ? 'Currently Assigned' : 'Reassign Case'}
            </button>
            <p className="help-text">
              <ShieldCheck size={14} /> Assignment passes If-Match: &quot;{record.version}&quot; and records an ASSIGNED event in the case timeline.
            </p>
          </Panel>

          {/* Human decision panel */}
          <Panel
            title="Record a decision"
            subtitle={`Optimistic concurrency · version ${record.version}`}
          >
            {ackRequired&&<Notice tone="warn">The case changed. Review the refreshed evidence before continuing.<button className="button button-ghost" onClick={()=>setAckRequired(false)}>I reviewed the updated evidence</button></Notice>}
            {!canReview&&<p className="help-text">Your role can read this case. An owner or reviewer records decisions.</p>}
            <label className="field-label" htmlFor="decision">
              Decision
            </label>
            <select
              id="decision"
              className="full-field"
              value={decision}
              disabled={!canReview || uncertain || ackRequired}
              onChange={(e) => setDecision(e.target.value as ReviewDecisionRequest['decision'])}
            >
              <option value="NEEDS_MORE_EVIDENCE">Needs more evidence</option>
              <option value="RESOLVE">Resolve</option>
              <option value="REOPEN">Reopen</option>
            </select>

            <label className="field-label" htmlFor="reason">
              Reason (minimum 5 characters)
            </label>
            <textarea
              id="reason"
              className="full-field"
              rows={4}
              value={reason}
              maxLength={2000}
              disabled={!canReview || uncertain || ackRequired}
              onChange={(e) => setReason(e.target.value)}
              placeholder="State verified findings and reason for decision…"
            />

            <button
              onClick={review}
              className="button button-primary button-full"
              disabled={!canReview || actionBusy || busy || Boolean(error) || ackRequired || reason.trim().length < 5 || reason.trim().length>2000}
            >
              <Send size={16} />
              {uncertain?'Retry unconfirmed decision':'Submit decision'}
            </button>
            <p className="help-text">
              <ShieldCheck size={14} /> Every decision requires explicit reason, passes If-Match: &quot;{record.version}&quot; and stable Idempotency-Key.
            </p>
          </Panel>
        </div>
      </div>

      {message && (
        <div className="toast" role="status">
          {message}
        </div>
      )}
    </>
  );
}

