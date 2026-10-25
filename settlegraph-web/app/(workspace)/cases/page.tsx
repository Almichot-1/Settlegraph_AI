'use client';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import { ArrowUpRight, RefreshCw, Search } from 'lucide-react';
import { api } from '@/lib/api';
import type { Case } from '@/generated/types';
import { shortDate } from '@/lib/format';
import { Badge, Empty, Failure, Loading, Panel, SectionHeading } from '@/components/ui';

export default function Cases() {
  const [items, setItems] = useState<Case[]>([]);
  const [status, setStatus] = useState('ALL');
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<unknown>(null);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const d = await api.listCases({ limit: 100 });
      setItems(d.items);
      setError(null);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void load();
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
          <button className="button button-ghost" onClick={load} disabled={busy}>
            <RefreshCw size={16} className={busy ? 'spin' : ''} />
            Refresh
          </button>
        }
      />
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
              placeholder="Search cases"
              aria-label="Search cases"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
        </div>

        {error ? <Failure error={error} onRetry={load} /> : null}
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
      </Panel>
    </>
  );
}

