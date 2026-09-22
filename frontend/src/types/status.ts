import type { ApprovalAction, EntityConfig } from './domain';

export type ArtifactState = 'registered' | 'stable' | 'treatment' | 'closed';
export const ALL_ARTIFACT_STATE: readonly ArtifactState[] = ['registered', 'stable', 'treatment', 'closed'];
export type ApprovalState = 'draft' | 'review' | 'pending_correction' | 'approved' | 'rejected';
export const ALL_APPROVAL_STATE: readonly ApprovalState[] = ['draft', 'review', 'pending_correction', 'approved', 'rejected'];

// 阶段审批在每个状态下当前角色可执行的操作。后端会再次强制校验角色与版本，
// 这里的配置只负责工作台按钮的显隐。
const APPROVAL_ACTIONS: Partial<Record<ApprovalState, ApprovalAction[]>> = {
  draft: [{ target: 'review', label: '提交审核', minRole: 'operator', tone: 'primary', placeholder: '填写提交复核的说明（必填）' }],
  review: [
    { target: 'approved', label: '复核通过', minRole: 'reviewer', tone: 'success', placeholder: '填写复核通过意见（必填）' },
    { target: 'rejected', label: '驳回', minRole: 'reviewer', tone: 'danger', placeholder: '填写驳回原因（必填）' },
    { target: 'pending_correction', label: '退回补正', minRole: 'reviewer', tone: 'warning', placeholder: '说明需要补正的内容（必填）' },
  ],
  pending_correction: [{ target: 'review', label: '提交补正', minRole: 'operator', tone: 'primary', placeholder: '填写补正说明，将开启新的复核批次（必填）' }],
};

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'artifact', path: 'artifacts', label: '文物', statuses: ['registered', 'stable', 'treatment', 'closed'] as const },
  { key: 'treatmentPlan', path: 'plans', label: '处理方案', statuses: ['draft', 'review', 'approved', 'completed'] as const },
  { key: 'materialTest', path: 'tests', label: '材料检测', statuses: ['planned', 'running', 'verified', 'invalid'] as const },
  {
    key: 'stageApproval', path: 'approvals', label: '阶段审批',
    statuses: ['draft', 'review', 'pending_correction', 'approved', 'rejected'] as const,
    actions: APPROVAL_ACTIONS,
  },
];
