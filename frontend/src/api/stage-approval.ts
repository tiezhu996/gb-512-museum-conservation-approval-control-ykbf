
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listStageApproval(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/approvals?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createStageApproval(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/approvals', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionStageApproval(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/approvals/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
// 复核人或管理员退回补正，意见必填。
export async function returnStageApprovalForCorrection(id: number, expectedVersion: number, opinion: string) {
  return request<DomainRecord>(`/approvals/${id}/return-correction`, {
    method: 'POST', body: JSON.stringify({ expectedVersion, opinion }),
  });
}
// 操作员提交补正说明，开启新的复核批次。
export async function submitStageApprovalCorrection(id: number, expectedVersion: number, opinion: string) {
  return request<DomainRecord>(`/approvals/${id}/correct`, {
    method: 'POST', body: JSON.stringify({ expectedVersion, opinion }),
  });
}
