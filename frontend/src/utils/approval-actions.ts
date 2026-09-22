import type { ApprovalAction, ApprovalOpinion, DomainRecord } from '../types/domain';

// Role ladder mirrors backend middleware roleRank.
const ROLE_RANK: Record<string, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };
const OPERATOR_RANK = ROLE_RANK.operator;
const REVIEWER_RANK = ROLE_RANK.reviewer;

// Static action catalogue. Labels are shown on the workbench buttons; each
// action carries whether a written opinion/explanation is mandatory.
const ACTIONS: Record<string, ApprovalAction> = {
  submit: { key: 'submit', label: '提交复核', target: 'review', requiresReason: true, minLength: 3 },
  approve: { key: 'approve', label: '复核通过', target: 'approved', requiresReason: true, minLength: 3 },
  reject: { key: 'reject', label: '驳回', target: 'rejected', danger: true, requiresReason: true, minLength: 3 },
  requestCorrection: { key: 'requestCorrection', label: '退回补正', target: 'correction', warning: true, requiresReason: true, minLength: 3 },
  submitCorrection: { key: 'submitCorrection', label: '提交补正', requiresReason: true, minLength: 3 },
};

// AvailableActions mirrors the backend state machine and RBAC rules:
// - draft -> review is an operator submission;
// - in review a reviewer/admin approves, rejects or requests correction;
// - correction (待补正) waits for an operator explanation;
// - approved is terminal.
export function availableActions(item: DomainRecord, role: string): ApprovalAction[] {
  const rank = ROLE_RANK[role] || 0;
  const isOperator = rank >= OPERATOR_RANK;
  const isReviewer = rank >= REVIEWER_RANK;
  switch (item.status) {
    case 'draft':
      return isOperator ? [ACTIONS.submit] : [];
    case 'review':
      return isReviewer ? [ACTIONS.approve, ACTIONS.requestCorrection, ACTIONS.reject] : [];
    case 'correction':
      return isOperator ? [ACTIONS.submitCorrection] : [];
    default:
      return [];
  }
}

export function actionPrompt(action: ApprovalAction): string {
  switch (action.key) {
    case 'submit': return '提交复核意见';
    case 'approve': return '复核通过意见';
    case 'reject': return '驳回意见';
    case 'requestCorrection': return '补正要求意见';
    case 'submitCorrection': return '补正说明';
    default: return '审批意见';
  }
}

// Current batch number: a correction resubmission opens the next review batch.
export function currentBatch(opinions: ApprovalOpinion[] | undefined): number {
  let batch = 1;
  for (const opinion of opinions || []) {
    if (opinion.kind === 'correction') batch = opinion.batch;
  }
  return batch;
}

export interface OpinionBatchGroup {
  batch: number;
  opinions: ApprovalOpinion[];
}

// Group opinions by review batch, preserving append order within each batch.
export function groupByBatch(opinions: ApprovalOpinion[] | undefined): OpinionBatchGroup[] {
  const groups = new Map<number, ApprovalOpinion[]>();
  for (const opinion of opinions || []) {
    const list = groups.get(opinion.batch) || [];
    list.push(opinion);
    groups.set(opinion.batch, list);
  }
  return [...groups.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([batch, list]) => ({ batch, opinions: list }));
}

export const OPINION_KIND_LABELS: Record<string, string> = {
  submit: '提交复核',
  decision: '复核决定',
  correction: '补正说明',
};
