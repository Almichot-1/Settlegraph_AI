'use client';
import Link from 'next/link';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ArrowUpRight, RefreshCw, Search } from 'lucide-react';
import { useWorkspaceSession,workspaceScope } from '@/lib/session';
import { activeScenario } from '@/lib/scenario';
import { api } from '@/lib/api';
import type { Case } from '@/generated/types';
import { shortDate } from '@/lib/format';
import { Badge, Empty, Failure, Loading, Panel, SectionHeading } from '@/components/ui';

export default function Cases() {
  const scope=workspaceScope(useWorkspaceSession());
  const [scenario,setScenario]=useState<string|null>(null);
  const [cursor,setCursor]=useState<string|null>(null);
  useEffect(()=>setScenario(activeScenario(scope)),[scope]);
  const [items, setItems] = useState<Case[]>([]);
  const [status, setStatus] = useState('ALL');
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(true);
  const generation=useRef(0);
  const [error, setError] = useState<unknown>(null);

  const load = useCallback(async (next?:string) => {
    const current=++generation.current;
    setBusy(true);
    try {
      const d = await api.listCases({ limit: 100,cursor:next,status:status==='ALL'?undefined:status as Case['status'],scenario_run_id:scenario??undefined });
      if(current!==generation.current)return;
      setItems(old=>next?[...old,...d.items]:d.items);setCursor(d.next_cursor);
      setError(null);
    } catch (e) {
      if(current===generation.current)setError(e);
    } finally {
      if(current===generation.current)setBusy(false);
    }
  }, [status,scenario]);

  useEffect(() => {
    void load();
    return ()=>{generation.current++;};
  }, [load]);

  const filtered = items.filter((c) => {
    const matchesStatus = status === 'ALL' || c.status === status;
    const searchTarget = `${c.title} ${c.kind} ${c.summary}`.toLowerCase();
    const matchesQuery = searchTarget.includes(query.trim().toLowerCase());
    return matchesStatus && matchesQuery;
  });

  return (
    <>
      <SectionHeading
        title="Investigation inbox"
        description="Evidence-based exception reviews and financial discrepancies."
        action={
          <button className="button button-ghost" onClick={()=>void load()} disabled={busy}>
            <RefreshCw size={16} className={busy ? 'spin' : ''} />
            Refresh
          </button>
        }
      />
      {scenario&&<p>Scenario {scenario.slice(0,8)} <button className="button button-ghost" onClick={()=>setScenario(null)}>Show all scenarios</button></p>}
      <Panel>
        <div className="filters">
          <div className="tab-row">
            {['ALL', 'OPEN', 'EVIDENCE_UPDATED', 'RESOLVED'].map((s) => (
              <button
                className={`tab ${status === s ? 'active' : ''}`}
                key={s}
                onClick={() => setStatus(s)}
              >
                {s.replaceAll('_', ' ')}
              </button>
            ))}
          </div>
          <div className="search-field">
            <Search size={16} />
            <input
              placeholder="Search loaded cases"
              aria-label="Search cases"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
        </div>

        {error ? <Failure error={error} onRetry={()=>void load()} /> : null}
        {busy && !items.length ? (
          <Loading />
        ) : filtered.length ? (
          <div className="case-list">
            {filtered.map((c) => (
              <Link className="case-list-item" key={c.id} href={`/cases/${c.id}`}>
                <div className="case-list-left">
                  <div className="case-list-head">
                    <strong>{c.title}</strong>
                    <Badge value={c.severity} />
                    <Badge value={c.kind} />
                  </div>
                  <p>{c.summary}</p>
                  <small>
                    Updated {shortDate(c.updated_at)} · {c.evidence_count} evidence record{c.evidence_count === 1 ? '' : 's'}
                  </small>
                </div>
                <div className="case-list-right">
                  <Badge value={c.status} />
                  <ArrowUpRight size={17} />
                </div>
              </Link>
            ))}
          </div>
        ) : (
          <Empty text="No cases for the selected filter." />
        )}
        {cursor&&<button className="button button-ghost" disabled={busy} onClick={()=>void load(cursor)}>Load more cases</button>}
      </Panel>
    </>
  );
}

