'use client';
import Link from 'next/link';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useParams } from 'next/navigation';
import { ArrowLeft, BrainCircuit, RefreshCw, Send, ShieldCheck, UserCheck } from 'lucide-react';
import { api } from '@/lib/api';
import { ApiRequestError } from '@/lib/client';
import type { Case, Evidence, CaseEvent, Investigation, ReviewDecisionRequest } from '@/generated/types';
import { formatMoney, makeKey, shortDate } from '@/lib/format';
import { Badge, Empty, Failure, Loading, Notice, Panel } from '@/components/ui';

const MAX_POLL_ATTEMPTS = 15;
const POLL_INTERVAL_MS = 2000;

const TEAM_MEMBERS = [
  { id: '33333333-3333-4333-8333-333333333333', name: 'Demo Owner', role: 'OWNER' },
  { id: '44444444-4444-4444-8444-444444444444', name: 'Abebe Kebede', role: 'REVIEWER' },
  { id: '55555555-5555-4555-8555-555555555555', name: 'Sara Haile', role: 'ANALYST' },
  { id: '66666666-6666-4666-8666-666666666666', name: 'Dawit Tadesse', role: 'REVIEWER' },
];

export default function CaseDetail() {
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
  const [decisionKey, setDecisionKey] = useState<string>(() => makeKey());
  const [pollingAiId, setPollingAiId] = useState<string | null>(null);
  const aiPollTimerRef = useRef<NodeJS.Timeout | null>(null);
  const aiPollCountRef = useRef(0);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const [c, e, ev, ai] = await Promise.all([
        api.getCase(caseId),
        api.listCaseEvidence(caseId),
        api.listCaseEvents(caseId),
        api.listInvestigations(caseId),
      ]);
      setRecord(c);
      setSelectedAssignee((prev) => prev || c.assigned_to || TEAM_MEMBERS[0].id);
      setEvidence(e.items);
      setEvents(ev.items);
      setInvestigations(ai.items);
      setError(null);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }, [caseId]);

  useEffect(() => {
    void load();
    return () => {
      if (aiPollTimerRef.current) clearTimeout(aiPollTimerRef.current);
    };
  }, [load]);

  // Bounded polling for asynchronous HTTP 202 investigation results
  const pollInvestigation = useCallback(
    (invId: string) => {
      if (aiPollTimerRef.current) clearTimeout(aiPollTimerRef.current);
      aiPollCountRef.current = 0;
      setPollingAiId(invId);

      const check = async () => {
        aiPollCountRef.current += 1;
        try {
          const inv = await api.getInvestigation(caseId, invId);
          const aiList = await api.listInvestigations(caseId);
          setInvestigations(aiList.items);

          if (inv.status === 'COMPLETED') {
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
          setMessage(err instanceof Error ? err.message : 'Error polling investigation');
          setPollingAiId(null);
        }
      };

      aiPollTimerRef.current = setTimeout(check, POLL_INTERVAL_MS);
    },
    [caseId]
  );

  async function investigate() {
    setActionBusy(true);
    setMessage('');
    try {
      const aiKey = makeKey();
      const ai = await api.createInvestigation(caseId, { mode: 'EVIDENCE_ANALYSIS' }, aiKey);
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
      const assignKey = makeKey();
      const updated = await api.assignCase(caseId, { assignee_id: selectedAssignee }, record.version, assignKey);
      setRecord(updated);
      const memberName = TEAM_MEMBERS.find((m) => m.id === selectedAssignee)?.name ?? selectedAssignee;
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
      const c = await api.recordCaseDecision(
        caseId,
        { decision, reason: cleanReason },
        record.version,
        decisionKey
      );
      setRecord(c);
      setMessage(`Decision "${decision}" recorded successfully with audit history.`);
      // Rotate stable idempotency key after successful mutation
      setDecisionKey(makeKey());
      await load();
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 412) {
        // HTTP 412 stale version conflict: notify user and refresh latest case state
        setMessage(
          'Case version conflict (HTTP 412): Someone updated this case. Latest version reloaded—please re-evaluate and re-submit your decision.'
        );
        setDecisionKey(makeKey());
        await load();
      } else {
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
              disabled={actionBusy || Boolean(pollingAiId)}
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
            subtitle={`Current owner: ${TEAM_MEMBERS.find((m) => m.id === (record.assigned_to ?? selectedAssignee))?.name ?? (record.assigned_to ?? 'Unassigned')}`}
            action={<UserCheck size={18} className="subtle-icon" />}
          >
            <label className="field-label" htmlFor="assignee-select">
              Assignee
            </label>
            <select
              id="assignee-select"
              className="full-field"
              value={selectedAssignee}
              onChange={(e) => setSelectedAssignee(e.target.value)}
            >
              {TEAM_MEMBERS.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name} ({m.role})
                </option>
              ))}
            </select>

            <button
              onClick={assign}
              className="button button-ghost button-full"
              style={{ marginTop: '10px' }}
              disabled={actionBusy || selectedAssignee === record.assigned_to}
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
            <label className="field-label" htmlFor="decision">
              Decision
            </label>
            <select
              id="decision"
              className="full-field"
              value={decision}
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
              onChange={(e) => setReason(e.target.value)}
              placeholder="State verified findings and reason for decision…"
            />

            <button
              onClick={review}
              className="button button-primary button-full"
              disabled={actionBusy || reason.trim().length < 5}
            >
              <Send size={16} />
              Submit decision
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

