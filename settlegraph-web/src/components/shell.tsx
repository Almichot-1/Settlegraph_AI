'use client';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Activity, ArrowLeftRight, BrainCircuit, Building2, ClipboardList, FileClock, LayoutDashboard, LogOut, Menu, ShieldCheck, X } from 'lucide-react';
import type { UserProfile } from '@/generated/types';
import { shorten } from '@/lib/format';

type Session = { user: UserProfile; organization_id: string };

const navigation = [
  { href: '/dashboard', label: 'Overview', icon: LayoutDashboard },
  { href: '/transactions', label: 'Payments', icon: ArrowLeftRight },
  { href: '/reconciliation', label: 'Reconciliation', icon: Activity },
  { href: '/cases', label: 'Investigations', icon: BrainCircuit },
  { href: '/simulation', label: 'Bank simulator', icon: Building2 },
  { href: '/audit', label: 'Audit trail', icon: FileClock },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const path = usePathname();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    fetch('/api/session', { cache: 'no-store' })
      .then(async (r) => {
        if (r.ok) setSession((await r.json()) as Session);
        else router.replace('/login');
      })
      .catch(() => router.replace('/login'));
  }, [router]);

  // Accessible keyboard handler: close drawer on Escape
  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [open]);

  async function logout() {
    await fetch('/api/session', { method: 'DELETE' });
    router.replace('/login');
    router.refresh();
  }

  const currentOrgName = session?.user.memberships[0]?.organization.name ?? 'Sample Merchant';

  return (
    <div className="app-shell">
      <aside
        id="workspace-sidebar"
        aria-label="Workspace navigation"
        className={`sidebar ${open ? 'sidebar-open' : ''}`}
      >
        <div className="brand">
          <div className="brand-mark" aria-hidden="true">
            <Activity size={21} />
          </div>
          <div>
            <strong>settlegraph<span className="brand-ai">.ai</span></strong>
            <small>PAYMENT INTELLIGENCE</small>
          </div>
          <button
            className="close-nav"
            aria-label="Close navigation menu"
            onClick={() => setOpen(false)}
          >
            <X size={20} />
          </button>
        </div>

        <div className="nav-label">WORKSPACE</div>
        <nav aria-label="Main navigation">
          {navigation.map((x) => {
            const Icon = x.icon;
            const isActive = path === x.href || path.startsWith(x.href + '/');
            return (
              <Link
                key={x.href}
                href={x.href}
                onClick={() => setOpen(false)}
                className={`nav-link ${isActive ? 'active' : ''}`}
                aria-current={isActive ? 'page' : undefined}
              >
                <Icon size={18} aria-hidden="true" />
                <span>{x.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="sidebar-bottom">
          <div className="sidebar-note">
            <ShieldCheck size={17} aria-hidden="true" />
            <div>
              <strong>Evidence-first</strong>
              <p>AI suggests. Verified records decide.</p>
            </div>
          </div>
          <div className="user-slot">
            <div className="user-avatar" aria-hidden="true">
              {(session?.user.display_name ?? 'D').slice(0, 1)}
            </div>
            <div>
              <strong>{session?.user.display_name ?? 'Demo user'}</strong>
              <small>{session?.user.memberships[0]?.role ?? 'Loading…'}</small>
            </div>
            <button title="Sign out" aria-label="Sign out" onClick={logout}>
              <LogOut size={17} />
            </button>
          </div>
        </div>
      </aside>

      {open && (
        <div
          className="sidebar-overlay"
          aria-hidden="true"
          onClick={() => setOpen(false)}
        />
      )}

      <div className="app-main">
        <header className="topbar">
          <button
            aria-label={open ? 'Close navigation' : 'Open navigation'}
            aria-expanded={open}
            aria-controls="workspace-sidebar"
            className="menu-icon"
            onClick={() => setOpen((prev) => !prev)}
          >
            <Menu size={21} />
          </button>
          <div className="breadcrumb">
            <ClipboardList size={16} aria-hidden="true" />
            <span>Merchant workspace</span>
            <span className="slash">/</span>
            <strong>
              {navigation.find((x) => path === x.href || path.startsWith(x.href + '/'))?.label ?? 'Overview'}
            </strong>
          </div>
          <div className="top-right">
            <div className="synthetic-badge" role="status" aria-label="Environment: Synthetic Data">
              <span className="demo-dot" aria-hidden="true" />
              <span className="demo-text">SYNTHETIC DATA</span>
            </div>
            <div className="org-pill" title={currentOrgName}>
              <Building2 size={14} aria-hidden="true" />
              <span className="org-pill-text">{shorten(currentOrgName, 22)}</span>
            </div>
          </div>
        </header>

        <main className="content" id="main-content">
          {children}
        </main>

        <footer className="footer">
          SettleGraph AI · Hackathon development environment · Simulated provider records only
        </footer>
      </div>
    </div>
  );
}
