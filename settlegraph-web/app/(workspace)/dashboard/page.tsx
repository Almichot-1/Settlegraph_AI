'use client';
import Link from 'next/link';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ArrowRight, ArrowUpRight, Building2, RefreshCw } from 'lucide-react';
import { api } from '@/lib/api';
import type { DashboardSummary, DashboardSeries, Case, Transaction } from '@/generated/types';
import { formatMoney, shortDate, shorten } from '@/lib/format';
import { Badge, Empty, Failure, Loading, Metric, Notice, Panel, SectionHeading } from '@/components/ui';

export default function Dashboard() {
  const [data, setData] = useState<DashboardSummary | null>(null);
  const [series, setSeries] = useState<DashboardSeries | null>(null);
  const [cases, setCases] = useState<Case[]>([]);
  const [tx, setTx] = useState<Transaction[]>([]);
  const generation=useRef(0);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(true);

  const load = useCallback(async () => {
    const current=++generation.current;
    setBusy(true);
    try {
      const d=await api.getDashboardSummary();
      const to=new Date(Date.parse(d.as_of)+1);
      const from=new Date(to.getTime()-7*86400000);
      const [s,c,t]=await Promise.all([
        api.getDashboardSeries({from:from.toISOString(),to:to.toISOString(),granularity:'DAY'}),
        api.listCases({limit:4}),api.listTransactions({limit:5})
      ]);
      if(current!==generation.current)return;
      setData(d);
      setSeries(s);
      setCases(c.items);
      setTx(t.items);
      setError(null);
    } catch (e) {
      if(current===generation.current)setError(e);
    } finally {
      if(current===generation.current)setBusy(false);
    }
  }, []);

  useEffect(() => {
    void load();
    return ()=>{generation.current++;};
  }, [load]);

  if (busy && !data) return <Loading text="Loading financial overview…" />;
  if (error && !data) return <Failure error={error} onRetry={load} />;

  const points = series?.points ?? [];
  const max = points.reduce((m, x) => {
    const p = BigInt(x.payments_accepted.minor_units);
    const s = BigInt(x.settlements_observed.minor_units);
    const local = p > s ? p : s;
    return local > m ? local : m;
  }, 1n);

  return (
    <>
      <SectionHeading
        title="Financial overview"
        description="Reconciliation intelligence across your connected simulation sources."
        action={
          <button
            type="button"
            onClick={load}
            disabled={busy}
            className="button button-ghost"
            aria-label="Refresh financial overview"
          >
            <RefreshCw size={16} className={busy ? 'spin' : ''} aria-hidden="true" />
            <span>{busy ? 'Refreshing…' : 'Refresh'}</span>
          </button>
        }
      />

      <Notice tone="info">
        Demonstration records only. Payment acceptance is an observed provider event and not proof of final bank settlement.
      </Notice>

      {error ? <div style={{ marginTop: 14 }}><Failure error={error} onRetry={load} /></div> : null}

      {/* 4 separate metrics: never combine orders, payments, settlements, and unmatched into a single balance */}
      <div className="metrics-grid">
        <Metric
          label="Orders expected"
          value={formatMoney(data?.orders_expected)}
          description="Recorded order obligations"
          accent="default"
        />
        <Metric
          label="Payments accepted"
          value={formatMoney(data?.payments_accepted)}
          description="Reported by provider observations"
          accent="info"
        />
        <Metric
          label="Settlements observed"
          value={formatMoney(data?.settlements_observed)}
          description="Matched imported bank records"
          accent="good"
        />
        <Metric
          label="Unmatched payments"
          value={formatMoney(data?.unmatched_payments)}
          description="Require reconciliation"
          accent="warn"
        />
      </div>

      <div className="dashboard-columns">
        <Panel
          title="Payment flow"
          subtitle="Accepted payments and observed settlements · last 7 days"
          action={
            <div className="tiny-key" aria-label="Chart legend">
              <span className="key-dot teal" aria-hidden="true" />
              <span>Payments</span>
              <span className="key-dot sand" aria-hidden="true" />
              <span>Settlements</span>
            </div>
          }
        >
          {points.length === 0 ? (
            <Empty text="No series points available for this period." />
          ) : (
            <div
              className="bar-chart"
              role="img"
              aria-label="Synthetic payment acceptance versus bank settlement flow chart"
            >
              {points.map((x) => {
                const pVal = BigInt(x.payments_accepted.minor_units);
                const sVal = BigInt(x.settlements_observed.minor_units);
                const a = max > 0n ? Number((pVal * 100n) / max) : 0;
                const b = max > 0n ? Number((sVal * 100n) / max) : 0;
                const aHeight = pVal > 0n ? Math.max(4, Math.min(100, a)) : 0;
                const bHeight = sVal > 0n ? Math.max(4, Math.min(100, b)) : 0;
                const dateLabel = new Date(x.bucket_start).toLocaleDateString('en-US', {
                  weekday: 'short',
                  month: 'short',
                  day: 'numeric',
                });
                const tooltip = `${dateLabel}: ${formatMoney(x.payments_accepted)} accepted · ${formatMoney(x.settlements_observed)} settled`;

                return (
                  <div className="bar-col" key={x.bucket_start} title={tooltip}>
                    <div className="bar-pair">
                      <div
                        className="bar teal-bar"
                        style={{ height: `${aHeight}%` }}
                        aria-label={`Payments: ${formatMoney(x.payments_accepted)}`}
                      />
                      <div
                        className="bar sand-bar"
                        style={{ height: `${bHeight}%` }}
                        aria-label={`Settlements: ${formatMoney(x.settlements_observed)}`}
                      />
                    </div>
                    <span>{new Date(x.bucket_start).toLocaleDateString('en-US', { weekday: 'short' })}</span>
                  </div>
                );
              })}
            </div>
          )}
        </Panel>

        <Panel
          title="Provider health"
          subtitle="Freshness of the latest available observations"
          action={<Building2 size={18} className="subtle-icon" aria-hidden="true" />}
        >
          <div className="provider-list">
            {(data?.providers ?? []).map((p) => (
              <div className="provider-item" key={p.id}>
                <div className="provider-icon" aria-hidden="true">
                  <Building2 size={17} />
                </div>
                <div className="provider-name">
                  <strong>{p.name}</strong>
                  <small>
                    {p.kind} · observed {p.last_observed_at ? shortDate(p.last_observed_at) : 'never'}
                  </small>
                </div>
                <div className="provider-status-badge">
                  <span className={`health-dot ${p.sync_status}`} aria-hidden="true" />
                  <Badge value={p.sync_status} />
                </div>
              </div>
            ))}
          </div>
          <div className="coverage-row">
            <span>Source coverage</span>
            <Badge value={data?.coverage.source_status ?? 'UNKNOWN'} />
          </div>
        </Panel>
      </div>

      <div className="dashboard-columns">
        <Panel
          title="Recent payment activity"
          action={
            <Link href="/transactions" className="text-link">
              <span>All payments</span>
              <ArrowRight size={15} aria-hidden="true" />
            </Link>
          }
        >
          {tx.length ? (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>Reference</th>
                    <th>Provider</th>
                    <th>Amount</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {tx.map((t) => (
                    <tr key={t.id}>
                      <td className="mono">{shorten(t.external_reference, 19)}</td>
                      <td>{t.provider_id}</td>
                      <td className="amount-cell">{formatMoney(t.amount)}</td>
                      <td>
                        <Badge value={t.state} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <Empty text="No recent payments recorded." />
          )}
        </Panel>

        <Panel
          title="Open investigations"
          subtitle={`${data?.open_cases ?? 0} unresolved cases`}
          action={
            <Link href="/cases" className="text-link">
              <span>View inbox</span>
              <ArrowUpRight size={15} aria-hidden="true" />
            </Link>
          }
        >
          {cases.length ? (
            <div className="case-preview">
              {cases
                .filter((c) => c.status !== 'RESOLVED')
                .slice(0, 4)
                .map((c) => (
                  <Link href={`/cases/${c.id}`} className="case-preview-row" key={c.id}>
                    <div>
                      <strong>{c.title}</strong>
                      <span>{shorten(c.summary, 80)}</span>
                    </div>
                    <Badge value={c.severity} />
                  </Link>
                ))}
            </div>
          ) : (
            <Empty text="No investigation cases in this workspace." />
          )}
        </Panel>
      </div>
    </>
  );
}
