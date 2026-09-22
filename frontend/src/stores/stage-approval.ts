import { defineStore } from 'pinia';
import {
  listStageApproval,
  createStageApproval,
  transitionStageApproval,
  returnStageApprovalForCorrection,
  submitStageApprovalCorrection,
} from '../api/stage-approval';
import type { DomainRecord, PageMeta } from '../types/domain';

interface StageApprovalState {
  items: DomainRecord[];
  meta: PageMeta;
  loading: boolean;
  error: string;
}

// 阶段审批状态仓库独立于通用 factory：退回补正与提交补正必须使用各自的专用
// 端点，由后端按角色、源状态和乐观锁再次校验。
export const useStageApprovalStore = defineStore('stageApproval', {
  state: (): StageApprovalState => ({
    items: [], meta: { page: 1, pageSize: 20, total: 0 }, loading: false, error: '',
  }),
  actions: {
    async load(path: string, search = '') {
      this.loading = true;
      this.error = '';
      try {
        const result = await listStageApproval(1, 20, search);
        this.items = result.data;
        this.meta = result.meta || { page: 1, pageSize: 20, total: result.data.length };
      } catch (error) {
        this.error = error instanceof Error ? error.message : String(error);
      } finally {
        this.loading = false;
      }
    },
    async createRecord(path: string, input: Partial<DomainRecord>) {
      this.loading = true;
      this.error = '';
      try {
        await createStageApproval(input);
        await this.load(path);
      } catch (error) {
        this.error = error instanceof Error ? error.message : String(error);
      } finally {
        this.loading = false;
      }
    },
    async transition(path: string, item: DomainRecord, status: string, reason = '') {
      this.loading = true;
      this.error = '';
      try {
        if (status === 'pending_correction') {
          await returnStageApprovalForCorrection(item.id, item.version, reason);
        } else if (item.status === 'pending_correction' && status === 'review') {
          await submitStageApprovalCorrection(item.id, item.version, reason);
        } else {
          await transitionStageApproval(item.id, status, item.version, reason);
        }
        await this.load(path);
      } catch (error) {
        // 无权限或版本过期时后端拒绝请求，这里回显错误且不改本地数据。
        this.error = error instanceof Error ? error.message : String(error);
      } finally {
        this.loading = false;
      }
    },
  },
});
