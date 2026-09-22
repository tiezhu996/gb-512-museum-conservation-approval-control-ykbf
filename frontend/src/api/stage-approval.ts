
import { request } from './client';
import type { CorrectionPayload, DomainRecord } from '../types/domain';

export async function listStageApproval(page = 1, pageSize = 20, search = '', status = '') {
  const query = new URLSearchParams({ page: String(page), pageSize: String(pageSize), search });
  if (status) query.set('status', status);
  return request<DomainRecord[]>(`/approvals?${query.toString()}`);
}
export async function createStageApproval(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/approvals', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionStageApproval(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/approvals/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
// Submit the operator correction explanation; the server reopens review in a
// new batch and appends the explanation as an immutable opinion.
export async function submitStageApprovalCorrection(id: number, payload: CorrectionPayload) {
  return request<DomainRecord>(`/approvals/${id}/corrections`, {
    method: 'POST', body: JSON.stringify(payload),
  });
}
