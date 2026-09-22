
export interface DomainRecord {
  id: number;
  code: string;
  name: string;
  status: string;
  version: number;
  description: string;
  facility: string;
  owner: string;
  category: string;
  riskLevel: 'low' | 'medium' | 'high' | 'critical';
  metricValue: number;
  metricUnit: string;
  effectiveAt: string;
  evidence: string;
  relatedCode: string;
  createdAt: string;
  updatedAt: string;
  opinions?: ApprovalOpinion[];
}

export interface ApprovalOpinion {
  id: number; stageApprovalId: number; version: number; batch: number; status: string;
  opinion: string; actor: string; requestId: string; createdAt: string;
}

export interface PageMeta { page: number; pageSize: number; total: number }
export interface ApiEnvelope<T> { data: T; error?: string; message?: string; meta?: PageMeta }
export interface UserSession { token: string; username: string; displayName: string; role: string; expiresIn: number }
export interface AuditLog {
  id: number; requestId: string; actor: string; action: string; entityType: string;
  entityId: number; beforeState: string; afterState: string; detail: string; createdAt: string;
}

export type ApprovalActionTone = 'primary' | 'success' | 'danger' | 'warning';

export interface ApprovalAction {
  /** Target approval status after this operation. */
  target: string;
  /** Button label shown on the workspace. */
  label: string;
  /** Minimum role allowed to trigger the operation. */
  minRole: 'operator' | 'reviewer' | 'admin';
  /** Element Plus button type. */
  tone: ApprovalActionTone;
  /** Placeholder for the mandatory opinion textarea. */
  placeholder: string;
}

export interface EntityConfig {
  key: string;
  path: string;
  label: string;
  statuses: readonly string[];
  /** Only 阶段审批 declares role-aware actions today. */
  actions?: Partial<Record<string, ApprovalAction[]>>;
}
