/** Typed browser API methods. This file covers the client-facing endpoints in the full contract. */
import * as T from '@/generated/types';
import { request, qs } from '@/lib/client';
import { makeKey } from '@/lib/format';

type PageFilter = { limit?: number; cursor?: string };
const esc = (v: string) => encodeURIComponent(v);
export const api = {
  getLiveness: () => request<T.HealthStatus>('/health/live'),
  getReadiness: () => request<T.HealthStatus>('/health/ready'),
  getMe: () => request<T.UserProfile>('/v1/me'),
  listProviders: (p:PageFilter={}) => request<T.ProviderList>('/v1/providers'+qs(p)),
  listAccounts: (p:PageFilter={}) => request<T.AccountPage>('/v1/accounts'+qs(p)),
  getDashboardSummary: () => request<T.DashboardSummary>('/v1/dashboard/summary'),
  getDashboardSeries: (p:{from:string;to:string;granularity?:'HOUR'|'DAY'}) => request<T.DashboardSeries>('/v1/dashboard/series'+qs(p)),
  listOrders: (p:PageFilter={}) => request<T.OrderPage>('/v1/orders'+qs(p)),
  getOrder: (id:string) => request<T.Order>('/v1/orders/'+esc(id)),
  listTransactions: (p:PageFilter&{provider_id?:string;scenario_run_id?:string}={}) => request<T.TransactionPage>('/v1/transactions'+qs(p)),
  getTransaction: (id:string) => request<T.Transaction>('/v1/transactions/'+esc(id)),
  listSettlements: (p:PageFilter={}) => request<T.SettlementPage>('/v1/settlements'+qs(p)),
  getSettlement: (id:string) => request<T.Settlement>('/v1/settlements/'+esc(id)),
  listReconciliationRuns: (p:PageFilter={}) => request<T.RunPage>('/v1/reconciliation/runs'+qs(p)),
  createReconciliationRun: (body:T.CreateRunRequest, idempotencyKey=makeKey()) => request<T.ReconciliationRun>('/v1/reconciliation/runs',{method:'POST',body,idempotencyKey}),
  getReconciliationRun: (id:string) => request<T.ReconciliationRun>('/v1/reconciliation/runs/'+esc(id)),
  listRunMatches: (id:string,p:PageFilter={}) => request<T.MatchPage>('/v1/reconciliation/runs/'+esc(id)+'/matches'+qs(p)),
  getMatchProof: (id:string) => request<T.MatchProof>('/v1/reconciliation/proofs/'+esc(id)),
  listCases: (p:PageFilter&{status?:T.Case['status'];assigned_to?:string;scenario_run_id?:string}={}) => request<T.CasePage>('/v1/cases'+qs(p)),
  getCase: (id:string) => request<T.Case>('/v1/cases/'+esc(id)),
  listCaseEvidence: (id:string,p:PageFilter={}) => request<T.EvidencePage>('/v1/cases/'+esc(id)+'/evidence'+qs(p)),
  listCaseEvents: (id:string,p:PageFilter={}) => request<T.CaseEventPage>('/v1/cases/'+esc(id)+'/events'+qs(p)),
  assignCase: (id:string,body:T.AssignCaseRequest,version:number,idempotencyKey=makeKey()) => request<T.Case>('/v1/cases/'+esc(id)+'/assign',{method:'POST',body,ifMatch:version,idempotencyKey}),
  recordCaseDecision: (id:string,body:T.ReviewDecisionRequest,version:number,idempotencyKey=makeKey()) => request<T.Case>('/v1/cases/'+esc(id)+'/decisions',{method:'POST',body,ifMatch:version,idempotencyKey}),
  createInvestigation: (id:string,body:T.InvestigationRequest={mode:'EVIDENCE_ANALYSIS'},idempotencyKey=makeKey()) => request<T.Investigation>('/v1/cases/'+esc(id)+'/investigations',{method:'POST',body,idempotencyKey}),
  listInvestigations: (id:string,p:PageFilter={}) => request<T.InvestigationPage>('/v1/cases/'+esc(id)+'/investigations'+qs(p)),
  getInvestigation: (caseId:string,id:string) => request<T.Investigation>('/v1/cases/'+esc(caseId)+'/investigations/'+esc(id)),
  getJob: (id:string) => request<T.Job>('/v1/jobs/'+esc(id)),
  listAuditEvents: (p:PageFilter={}) => request<T.AuditEventPage>('/v1/audit-events'+qs(p)),
  listDemoScenarios: () => request<T.ScenarioList>('/v1/demo/scenarios'),
  createScenarioRun: (key:T.ScenarioDefinition['key'],seed:number,idempotencyKey=makeKey()) => request<T.ScenarioRun>('/v1/demo/scenarios/'+esc(key)+'/runs',{method:'POST',body:{seed},idempotencyKey}),
  getScenarioRun: (id:string) => request<T.ScenarioRun>('/v1/demo/scenario-runs/'+esc(id)),
  advanceScenarioRun: (id:string,expected_step:number,idempotencyKey=makeKey()) => request<T.ScenarioRun>('/v1/demo/scenario-runs/'+esc(id)+'/advance',{method:'POST',body:{expected_step},idempotencyKey}),
  listImports: (p:PageFilter={}) => request<T.ImportPage>('/v1/imports'+qs(p)),
  createImport: (body:T.CreateImportRequest,idempotencyKey=makeKey()) => request<T.ImportUploadSession>('/v1/imports',{method:'POST',body,idempotencyKey}),
  getImport: (id:string) => request<T.Import>('/v1/imports/'+esc(id)),
  completeImport: (id:string,body:T.CompleteImportRequest,idempotencyKey=makeKey()) => request<T.Import>('/v1/imports/'+esc(id)+'/complete',{method:'POST',body,idempotencyKey}),
  // acceptBankSimEvent is intentionally absent: bank-to-bank webhook requires a server-held signature.
};
