'use client';
import { createContext, useContext } from 'react';
import type { UserProfile } from '@/generated/types';
export type WorkspaceSession = {user:UserProfile;organization_id:string};
export const SessionContext = createContext<WorkspaceSession|null>(null);
export const useWorkspaceSession = () => useContext(SessionContext);
export function activeRole(session:WorkspaceSession|null) {
  return session?.user.memberships.find(m=>m.organization.id===session.organization_id)?.role;
}
export function workspaceScope(session:WorkspaceSession|null):string {
  return session ? session.organization_id+':'+session.user.id : 'unavailable';
}
