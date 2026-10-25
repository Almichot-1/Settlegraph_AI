'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { CheckCircle2, FileCheck, Layers, Play, RefreshCw, ShieldCheck, X } from 'lucide-react';
import { useWorkspaceSession, workspaceScope } from '@/lib/session';
import { activeScenario } from '@/lib/scenario';
import { runMutation } from '@/lib/mutations';
import { allPages } from '@/lib/client';
import { api } from '@/lib/api';
import type { Match, MatchProof, ReconciliationRun, CreateRunRequest } from '@/generated/types';
import { formatMoney, shortDate, shorten } from '@/lib/format';
import { Badge, Empty, Failure, Loading, Notice, Panel, SectionHeading } from '@/components/ui';

const MAX_POLL_ATTEMPTS = 15;
const POLL_INTERVAL_MS = 2000;

export default function Reconciliation() {
  const scope=workspaceScope(useWorkspaceSession());
  const [scenario,setScenario]=useState<string|null>(null);
  const [runScope,setRunScope]=useState<CreateRunRequest['scope']>('LATEST_OBSERVATIONS');
  useEffect(()=>{const id=activeScenario(scope);setScenario(id);if(id)setRunScope('SCENARIO_RUN');},[scope]);
  const [nextCursor,setNextCursor]=useState<string|null>(null);
  const [runs, setRuns] = useState<ReconciliationRun[]>([]);
  const [busy, setBusy] = useState(true);
  const [launching, setLaunching] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [message, setMessage] = useState('');
  const [pollingRunId, setPollingRunId] = useState<string | null>(null);
  const pollTimerRef = useRef<NodeJS.Timeout | null>(null);
  const pollCountRef = useRef(0);

  // Match explorer & proof inspector state
  const [selectedRun, setSelectedRun] = useState<ReconciliationRun | null>(null);
  const [matches, setMatches] = useState<Match[]>([]);
  const [loadingMatches, setLoadingMatches] = useState(false);
  const [stageFilter, setStageFilter] = useState<'ALL' | 'ORDER_PAYMENT' | 'PAYMENT_SETTLEMENT'>('ALL');
  const [selectedProof, setSelectedProof] = useState<MatchProof | null>(null);
  const [loadingProof, setLoadingProof] = useState(false);

  const load = useCallback(async (cursor?:string) => {
    setBusy(true);
    try {
      const p = await api.listReconciliationRuns({ limit: 50,cursor });
      setRuns(old=>(cursor?[...old,...p.items]:p.items).sort((a,b)=>b.created_at.localeCompare(a.created_at)));
      setNextCursor(p.next_cursor);
      setError(null);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void load();
    return () => {
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
    };
  }, [load]);

  // Handle escape key to close modals
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (selectedProof) {
          setSelectedProof(null);
        } else if (selectedRun) {
          setSelectedRun(null);
        }
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedProof, selectedRun]);

  // Bounded polling for asynchronous HTTP 202 jobs until terminal state (COMPLETED / FAILED)
  const pollRun = useCallback((runId: string) => {
    if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
    pollCountRef.current = 0;
    setPollingRunId(runId);

    const check = async () => {
      pollCountRef.current += 1;
      try {
        const run = await api.getReconciliationRun(runId);
        const list = await api.listReconciliationRuns({ limit: 50 });
        setRuns(list.items);

        if (run.status === 'COMPLETED') {
          setMessage(`Reconciliation run ${runId.slice(0, 8)} completed.`);
          setPollingRunId(null);
          return;
        }
        if (run.status === 'FAILED') {
          setMessage(`Reconciliation run ${runId.slice(0, 8)} failed.`);
          setPollingRunId(null);
          return;
        }

        if (pollCountRef.current < MAX_POLL_ATTEMPTS) {
          pollTimerRef.current = setTimeout(check, POLL_INTERVAL_MS);
        } else {
          setMessage(`Reconciliation run ${runId.slice(0, 8)} is still processing in background. Refresh anytime to check status.`);
          setPollingRunId(null);
        }
      } catch (err) {
        setMessage(err instanceof Error ? err.message : 'Error polling reconciliation run');
        setPollingRunId(null);
      }
    };

    pollTimerRef.current = setTimeout(check, POLL_INTERVAL_MS);
  }, []);

  async function create() {
    setLaunching(true);
    setMessage('');
    try {
      const body:CreateRunRequest=runScope==='SCENARIO_RUN'?{scope:runScope,scenario_run_id:scenario}:{scope:runScope};
      const r=await runMutation(scope+':reconciliation:create',body,undefined,m=>api.createReconciliationRun(m.body as CreateRunRequest,m.key));
      await load();

      if (r.status === 'COMPLETED') {
        setMessage(`Reconciliation run ${r.id.slice(0, 8)} completed.`);
      } else if (r.status === 'FAILED') {
        setMessage(`Reconciliation run ${r.id.slice(0, 8)} failed.`);
      } else {
        setMessage(`Reconciliation run ${r.id.slice(0, 8)} queued (HTTP 202). Polling for completion…`);
        pollRun(r.id);
      }
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Run failed');
    } finally {
      setLaunching(false);
    }
  }

  async function inspectRun(run: ReconciliationRun) {
    setSelectedRun(run);
    setSelectedProof(null);
    setLoadingMatches(true);
    try {
      setMatches(await allPages(cursor=>api.listRunMatches(run.id,{limit:100,cursor})));
    } catch {
      setMatches([]);
    } finally {
      setLoadingMatches(false);
    }
  }

  async function viewProof(proofId: string) {
    setLoadingProof(true);
    try {
      const proof = await api.getMatchProof(proofId);
      setSelectedProof(proof);
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Could not load match proof');
    } finally {
      setLoadingProof(false);
    }
  }

  const filteredMatches = matches.filter(
    (m) => stageFilter === 'ALL' || m.stage === stageFilter
  );

  return (
    <>
      <SectionHeading
        title="Reconciliation runs"
        description="Compare observed orders, payments, and settlements using verifiable matching rules."
        action={
          <button className="button button-ghost" onClick={()=>void load()} disabled={busy || Boolean(pollingRunId)}>
            <RefreshCw size={16} className={busy || pollingRunId ? 'spin' : ''} />
            Refresh
          </button>
        }
      />
      <Notice>These are transaction matching runs, not bank settlement instructions. No money is moved by this application.</Notice>

      <Panel title="New run" subtitle="Process the latest imported or simulated observations">
        <label htmlFor="run-scope">Evidence scope</label>
        <select id="run-scope" value={runScope} onChange={e=>setRunScope(e.target.value as CreateRunRequest['scope'])}>
          {scenario&&<option value="SCENARIO_RUN">Selected scenario {scenario.slice(0,8)}</option>}
          <option value="LATEST_OBSERVATIONS">Unscoped imported observations</option>
        </select>
        <div className="flex-between">
          <div className="help-text">A real run may remain queued or running while backend workers process events.</div>
          <button className="button button-primary" onClick={create} disabled={launching || Boolean(pollingRunId)}>
            <Play size={16} />
            {launching ? 'Starting…' : pollingRunId ? 'Processing run…' : 'Start reconciliation'}
          </button>
        </div>
        {message && <div className="inline-message" role="status">{message}</div>}
      </Panel>

      {nextCursor&&<button className="button button-ghost" disabled={busy} onClick={()=>void load(nextCursor)}>Load more runs</button>}
      <Panel title="Run history" subtitle="Click any run to inspect verifiable matches and deterministic proofs">
        <div className="table-scroll">
          {error ? <Failure error={error} onRetry={()=>void load()} /> : null}
          {busy && !runs.length ? (
            <Loading />
          ) : runs.length ? (
            <table>
              <thead>
                <tr>
                  <th>Run ID</th>
                  <th>Status</th>
                  <th>Order ↔ payment</th>
                  <th>Payment ↔ settlement</th>
                  <th>Unmatched</th>
                  <th>Started</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((r) => (
                  <tr
                    key={r.id}
                    className="clickable-row"
                    tabIndex={0}
                    role="button"
                    onClick={() => inspectRun(r)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        inspectRun(r);
                      }
                    }}
                  >
                    <td className="mono">{r.id.slice(0, 13)}…</td>
                    <td><Badge value={r.status} /></td>
                    <td>{r.matched_order_payment}</td>
                    <td>{r.matched_payment_settlement}</td>
                    <td>{r.unmatched_count}</td>
                    <td>{shortDate(r.created_at)}</td>
                    <td>
                      <button
                        className="button button-ghost"
                        style={{ padding: '4px 10px', fontSize: '12px' }}
                        onClick={(e) => {
                          e.stopPropagation();
                          inspectRun(r);
                        }}
                      >
                        <Layers size={13} /> Matches
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <Empty text="No reconciliation runs yet. Start one to see its results." />
          )}
        </div>
      </Panel>

      {/* Match Explorer Modal */}
      {selectedRun && (
        <div className="modal-backdrop" role="presentation" onClick={() => setSelectedRun(null)}>
          <div
            className="detail-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Reconciliation matches"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: '780px' }}
          >
            <button className="modal-close" onClick={() => setSelectedRun(null)} aria-label="Close dialog">
              <X size={18} />
            </button>

            <div className="eyebrow">RECONCILIATION RUN #{selectedRun.id.slice(0, 8)}</div>
            <h2>Deterministic Match Explorer</h2>
            <p className="subtitle">
              Scope: <strong>{selectedRun.scope}</strong> · Rules version: <strong>{selectedRun.rules_version}</strong> · Started: {shortDate(selectedRun.created_at)}
            </p>

            <div className="metrics-grid" style={{ marginBottom: '20px' }}>
              <div className="metric metric-info">
                <span>Order ↔ Payment</span>
                <strong>{selectedRun.matched_order_payment}</strong>
                <small>Matched pairs</small>
              </div>
              <div className="metric metric-good">
                <span>Payment ↔ Settlement</span>
                <strong>{selectedRun.matched_payment_settlement}</strong>
                <small>Settled matches</small>
              </div>
              <div className="metric metric-warn">
                <span>Unmatched</span>
                <strong>{selectedRun.unmatched_count}</strong>
                <small>Open discrepancies</small>
              </div>
            </div>

            <div className="tab-row" style={{ marginBottom: '14px' }}>
              {(['ALL', 'ORDER_PAYMENT', 'PAYMENT_SETTLEMENT'] as const).map((stage) => (
                <button
                  key={stage}
                  className={`tab ${stageFilter === stage ? 'active' : ''}`}
                  onClick={() => setStageFilter(stage)}
                >
                  {stage.replace('_', ' ')}
                </button>
              ))}
            </div>

            {loadingMatches ? (
              <Loading text="Loading run matches…" />
            ) : filteredMatches.length ? (
              <div className="table-scroll" style={{ maxHeight: '320px', overflowY: 'auto' }}>
                <table>
                  <thead>
                    <tr>
                      <th>Stage</th>
                      <th>Left evidence</th>
                      <th>Right evidence</th>
                      <th>Amount</th>
                      <th>Rule</th>
                      <th>Proof</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredMatches.map((m) => (
                      <tr key={m.id}>
                        <td><Badge value={m.stage} /></td>
                        <td className="mono" title={m.left_evidence_id}>{shorten(m.left_evidence_id, 12)}</td>
                        <td className="mono" title={m.right_evidence_id}>{shorten(m.right_evidence_id, 12)}</td>
                        <td className="amount-cell">{formatMoney(m.allocated_amount)}</td>
                        <td><small>{m.rule.replaceAll('_', ' ')}</small></td>
                        <td>
                          <button
                            className="button button-ghost"
                            style={{ padding: '3px 8px', fontSize: '11px' }}
                            onClick={() => viewProof(m.proof_id)}
                            disabled={loadingProof}
                          >
                            <FileCheck size={13} /> View proof
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <Empty text="No matches found in this stage for this run." />
            )}
          </div>
        </div>
      )}

      {/* Match Proof Inspector Sub-Modal */}
      {selectedProof && (
        <div className="modal-backdrop" role="presentation" onClick={() => setSelectedProof(null)} style={{ zIndex: 1100 }}>
          <div
            className="detail-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Match proof verification"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: '580px', border: '1px solid var(--accent)' }}
          >
            <button className="modal-close" onClick={() => setSelectedProof(null)} aria-label="Close dialog">
              <X size={18} />
            </button>

            <div className="eyebrow">VERIFIED MATCH PROOF</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginTop: '4px' }}>
              <ShieldCheck size={24} style={{ color: 'var(--good)' }} />
              <h2 style={{ margin: 0 }}>Proof #{selectedProof.id.slice(0, 8)}</h2>
            </div>
            <p className="subtitle" style={{ marginTop: '4px' }}>
              Stage: <strong>{selectedProof.stage.replaceAll('_', ' ')}</strong> · Rule engine: <strong>{selectedProof.rule_version}</strong>
            </p>

            <div style={{ margin: '14px 0' }}>
              <Badge value={selectedProof.result} />
            </div>

            <div style={{ background: 'var(--panel-subtle)', borderRadius: '8px', padding: '14px', marginBottom: '16px' }}>
              <strong style={{ fontSize: '13px', display: 'block', marginBottom: '8px' }}>
                Verifiable Checks Executed
              </strong>
              <ul style={{ margin: 0, paddingLeft: '18px', display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {selectedProof.checks.map((chk, idx) => (
                  <li key={idx} style={{ fontSize: '13px', display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <CheckCircle2 size={14} style={{ color: 'var(--good)', flexShrink: 0 }} />
                    <code>{chk}</code>
                  </li>
                ))}
              </ul>
            </div>

            <div style={{ background: 'var(--panel-subtle)', borderRadius: '8px', padding: '14px' }}>
              <strong style={{ fontSize: '13px', display: 'block', marginBottom: '8px' }}>
                Source Evidence Identifiers
              </strong>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {selectedProof.source_evidence_ids.map((id, idx) => (
                  <span key={idx} className="mono" style={{ background: 'var(--bg)', padding: '3px 8px', borderRadius: '4px', fontSize: '12px' }}>
                    {id}
                  </span>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
