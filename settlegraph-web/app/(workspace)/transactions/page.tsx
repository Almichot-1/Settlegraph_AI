'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { RefreshCw, Search, X } from 'lucide-react';
import { useWorkspaceSession,workspaceScope } from '@/lib/session';
import { activeScenario } from '@/lib/scenario';
import { api } from '@/lib/api';
import type { Transaction, Provider } from '@/generated/types';
import { formatMoney, shortDate } from '@/lib/format';
import { Badge, Empty, Failure, Loading, Panel, SectionHeading } from '@/components/ui';

export default function Transactions() {
  const scope=workspaceScope(useWorkspaceSession());
  const [scenario,setScenario]=useState<string|null>(null);
  useEffect(()=>setScenario(activeScenario(scope)),[scope]);
  const [items, setItems] = useState<Transaction[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [provider, setProvider] = useState('');
  const [search, setSearch] = useState('');
  const [cursor, setCursor] = useState<string | null>(null);
  const [busy, setBusy] = useState(true);
  const generation=useRef(0);
  const [error, setError] = useState<unknown>(null);
  const [selected, setSelected] = useState<Transaction | null>(null);

  const load = useCallback(
    async (next?: string) => {
      const current=++generation.current;
      setBusy(true);
      try {
        const [t, p] = await Promise.all([
          api.listTransactions({ limit: 30, provider_id: provider || undefined, scenario_run_id:scenario??undefined, cursor: next }),
          api.listProviders(),
        ]);
        if(current!==generation.current)return;
        setItems((previous) => (next ? [...previous, ...t.items] : t.items));
        setCursor(t.next_cursor);
        setProviders(p.items);
        setError(null);
      } catch (e) {
        if(current===generation.current)setError(e);
      } finally {
        if(current===generation.current)setBusy(false);
      }
    },
    [provider,scenario]
  );

  useEffect(() => {
    void load();
    return ()=>{generation.current++;};
  }, [load]);

  // Accessible keyboard handler: close modal dialog on Escape
  useEffect(() => {
    if (!selected) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setSelected(null);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selected]);

  const shown = items.filter((t) =>
    (t.external_reference + ' ' + t.provider_id + ' ' + (t.order_reference ?? ''))
      .toLowerCase()
      .includes(search.toLowerCase())
  );

  return (
    <>
      {scenario&&<p>Scenario {scenario.slice(0,8)} <button className="button button-ghost" onClick={()=>setScenario(null)}>Show all scenarios</button></p>}
      <SectionHeading
        title="Payment activity"
        description="Observed payment records from simulated financial providers."
        action={
          <button
            type="button"
            className="button button-ghost"
            onClick={() => load()}
            disabled={busy}
            aria-label="Refresh transactions"
          >
            <RefreshCw size={16} className={busy ? 'spin' : ''} aria-hidden="true" />
            <span>Refresh</span>
          </button>
        }
      />

      <Panel>
        <div className="filters">
          <div className="search-field">
            <Search size={17} aria-hidden="true" />
            <input
              aria-label="Search transactions"
              placeholder="Search reference, provider or order…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <select
            aria-label="Filter provider"
            value={provider}
            onChange={(e) => setProvider(e.target.value)}
          >
            <option value="">All providers</option>
            {providers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </div>

        {error ? <Failure error={error} onRetry={() => load()} /> : null}

        {busy && !items.length ? (
          <Loading text="Loading payment activity…" />
        ) : shown.length ? (
          <div className="table-scroll">
            <table aria-label="Transactions table">
              <thead>
                <tr>
                  <th>Transaction reference</th>
                  <th>Provider</th>
                  <th>Order</th>
                  <th>Amount</th>
                  <th>State</th>
                  <th>Observed</th>
                </tr>
              </thead>
              <tbody>
                {shown.map((t) => (
                  <tr
                    key={t.id}
                    onClick={() => setSelected(t)}
                    className="clickable-row"
                    tabIndex={0}
                    role="button"
                    aria-label={`View evidence for transaction ${t.external_reference}`}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setSelected(t);
                      }
                    }}
                  >
                    <td className="mono">{t.external_reference}</td>
                    <td>{t.provider_id}</td>
                    <td className="mono">{t.order_reference ?? '—'}</td>
                    <td className="amount-cell">{formatMoney(t.amount)}</td>
                    <td>
                      <Badge value={t.state} />
                    </td>
                    <td>{shortDate(t.observed_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty text="No transactions match these filters." />
        )}

        {cursor && (
          <button
            type="button"
            className="button button-ghost load-more"
            onClick={() => load(cursor)}
            disabled={busy}
            aria-label="Load more transactions"
          >
            {busy ? 'Loading…' : 'Load more'}
          </button>
        )}
      </Panel>

      {selected && (
        <div
          className="modal-backdrop"
          role="presentation"
          onClick={() => setSelected(null)}
        >
          <div
            className="detail-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="modal-tx-title"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              className="modal-close"
              aria-label="Close dialog"
              onClick={() => setSelected(null)}
            >
              <X size={18} aria-hidden="true" />
            </button>
            <h2 id="modal-tx-title">Transaction evidence</h2>
            <p className="subtitle">
              This is an observed provider record from a simulated feed, not proof of a live bank balance.
            </p>
            <dl className="details-grid">
              <dt>Reference</dt>
              <dd className="mono">{selected.external_reference}</dd>
              <dt>Provider</dt>
              <dd>{selected.provider_id}</dd>
              <dt>Order</dt>
              <dd className="mono">{selected.order_reference ?? '—'}</dd>
              <dt>Amount</dt>
              <dd className="amount-cell">{formatMoney(selected.amount)}</dd>
              <dt>State</dt>
              <dd>
                <Badge value={selected.state} />
              </dd>
              <dt>Attestation</dt>
              <dd>
                <Badge value={selected.source_attestation} />
              </dd>
              <dt>Occurred</dt>
              <dd>{shortDate(selected.occurred_at)}</dd>
              <dt>Observed</dt>
              <dd>{shortDate(selected.observed_at)}</dd>
            </dl>
          </div>
        </div>
      )}
    </>
  );
}
