import type { EntityConfig } from './domain';

export type ArtifactState = 'registered' | 'stable' | 'treatment' | 'closed';
export const ALL_ARTIFACT_STATE: readonly ArtifactState[] = ['registered', 'stable', 'treatment', 'closed'];
export type ApprovalState = 'draft' | 'review' | 'correction' | 'approved' | 'rejected';
export const ALL_APPROVAL_STATE: readonly ApprovalState[] = ['draft', 'review', 'correction', 'approved', 'rejected'];

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'artifact', path: 'artifacts', label: '文物', statuses: ['registered', 'stable', 'treatment', 'closed'] as const },
  { key: 'treatmentPlan', path: 'plans', label: '处理方案', statuses: ['draft', 'review', 'approved', 'completed'] as const },
  { key: 'materialTest', path: 'tests', label: '材料检测', statuses: ['planned', 'running', 'verified', 'invalid'] as const },
  { key: 'stageApproval', path: 'approvals', label: '阶段审批', statuses: ['draft', 'review', 'correction', 'approved', 'rejected'] as const }
];

// Chinese labels and tones for the stage-approval workbench, including 待补正.
export const APPROVAL_STATUS_LABELS: Record<string, string> = {
  draft: '草稿',
  review: '待复核',
  correction: '待补正',
  approved: '复核通过',
  rejected: '已驳回',
};
