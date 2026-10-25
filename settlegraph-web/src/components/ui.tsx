'use client';
import type { ReactNode } from 'react';
import { AlertCircle, CheckCircle2, Clock3, Info, RefreshCw } from 'lucide-react';
import { ApiRequestError } from '@/lib/client';

export function SectionHeading({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="section-heading">
      <div>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {action && <div className="section-action">{action}</div>}
    </div>
  );
}

export function Panel({
  title,
  subtitle,
  children,
  action,
  className = '',
}: {
  title?: string;
  subtitle?: string;
  children: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <section className={`panel ${className}`}>
      <div className="panel-top">
        {title && (
          <div>
            <h2>{title}</h2>
            {subtitle && <p>{subtitle}</p>}
          </div>
        )}
        {action && <div>{action}</div>}
      </div>
      {children}
    </section>
  );
}

export function Metric({
  label,
  value,
  description,
  accent = 'default',
}: {
  label: string;
  value: string;
  description?: string;
  accent?: 'default' | 'good' | 'warn' | 'info' | 'bad';
}) {
  return (
    <div className={`metric metric-${accent}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{description ?? 'From available records'}</small>
    </div>
  );
}

export function Badge({ value }: { value: string }) {
  const v = value.toUpperCase();
  const tone =
    ['COMPLETED', 'RESOLVED', 'HEALTHY', 'ACCEPTED', 'COMPLETE', 'READY', 'SETTLED', 'MATCHED', 'VERIFIED', 'UP'].includes(v)
      ? 'good'
      : ['OUTAGE', 'FAILED', 'HIGH', 'CRITICAL', 'REJECTED', 'UNMATCHED', 'DISPUTED', 'ERROR', 'NOT_READY'].includes(v)
      ? 'bad'
      : ['OPEN', 'MEDIUM', 'INCOMPLETE_SOURCE', 'STALE', 'EVIDENCE_UPDATED', 'QUEUED', 'RUNNING', 'PREPARING', 'PENDING', 'NEEDS_MORE_EVIDENCE', 'NEVER_SYNCED', 'PARTIAL', 'DEGRADED'].includes(v)
      ? 'warn'
      : ['LOW', 'INVESTIGATING', 'INFO'].includes(v)
      ? 'info'
      : 'neutral';

  return <span className={`badge badge-${tone}`}>{value.replaceAll('_', ' ')}</span>;
}

export function Notice({ children, tone = 'info' }: { children: ReactNode; tone?: 'info' | 'warn' | 'error' }) {
  return (
    <div className={`notice notice-${tone}`} role={tone === 'error' ? 'alert' : 'note'}>
      {tone === 'warn' ? (
        <AlertCircle size={17} aria-hidden="true" />
      ) : tone === 'error' ? (
        <AlertCircle size={17} aria-hidden="true" />
      ) : (
        <Info size={17} aria-hidden="true" />
      )}
      <div>{children}</div>
    </div>
  );
}

export function Loading({ text = 'Loading data…' }: { text?: string }) {
  return (
    <div className="loading" role="status" aria-live="polite">
      <RefreshCw size={18} className="spin" aria-hidden="true" />
      <span>{text}</span>
    </div>
  );
}

export function Empty({ text = 'No records available.' }: { text?: string }) {
  return (
    <div className="empty" role="status">
      <Clock3 size={19} aria-hidden="true" />
      <span>{text}</span>
    </div>
  );
}

export function Failure({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const e =
    error instanceof ApiRequestError
      ? `${error.code}: ${error.message}`
      : error instanceof Error
      ? error.message
      : 'Unknown error';

  return (
    <div className="failure" role="alert">
      <AlertCircle size={18} aria-hidden="true" />
      <div>
        <strong>Unable to load</strong>
        <p>{e}</p>
      </div>
      {onRetry && (
        <button type="button" className="button button-ghost" onClick={onRetry} aria-label="Retry operation">
          Retry
        </button>
      )}
    </div>
  );
}

export function InfoLine({ children }: { children: ReactNode }) {
  return (
    <div className="info-line">
      <CheckCircle2 size={15} aria-hidden="true" />
      <div>{children}</div>
    </div>
  );
}
