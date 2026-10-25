import type { Money } from '@/generated/types';

/** ETB minor-unit strings are formatted with BigInt to avoid floating-point errors. */
export function formatMoney(value: Money | null | undefined): string {
  if (!value || typeof value.minor_units !== 'string') return '—';
  const clean = value.minor_units.trim();
  if (!clean || !/^-?\d+$/.test(clean)) return '—';
  const n = BigInt(clean);
  const abs = n < 0n ? -n : n;
  const whole = (abs / 100n).toLocaleString('en-US');
  const cents = (abs % 100n).toString().padStart(2, '0');
  return `${n < 0n ? '−' : ''}${whole}.${cents} ${value.currency ?? 'ETB'}`;
}

export function shortDate(s: string | null | undefined): string {
  if (!s) return 'Not observed';
  const date = new Date(s);
  return Number.isNaN(date.valueOf()) ? 'Unknown' : date.toLocaleString('en-ET', {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: 'Africa/Addis_Ababa',
  });
}

export const shorten = (s: string, max = 24) => (s.length > max ? `${s.slice(0, max - 3)}…` : s);
export const makeKey = () => globalThis.crypto?.randomUUID?.() ?? `sg-${Date.now()}-${Math.random().toString(36).slice(2)}`;
