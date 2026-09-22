import { defineStore } from 'pinia';
import {
  createStageApproval,
  listStageApproval,
  submitStageApprovalCorrection,
  transitionStageApproval,
} from '../api/stage-approval';
import type { ApprovalAction, DomainRecord, PageMeta } from '../types/domain';

interface StageApprovalState {
  items: DomainRecord[];
  meta: PageMeta;
  loading: boolean;
  error: string;
  search: string;
  statusFilter: string;
}

// Dedicated store for 阶段审批. Unlike the generic factory it preserves the
// reviewer/operator action split and carries written opinions end to end.
export const useStageApprovalStore = defineStore('stageApproval', {
  state: (): StageApprovalState => ({
    items: [],
    meta: { page: 1, pageSize: 20, total: 0 },
    loading: false,
    error: '',
    search: '',
    statusFilter: '',
  }),
  actions: {
    async load(search?: string, status?: string) {
      this.loading = true;
      this.error = '';
      this.search = search ?? this.search;
      this.statusFilter = status ?? this.statusFilter;
      try {
        const result = await listStageApproval(1, 20, this.search, this.statusFilter);
        this.items = result.data;
        this.meta = result.meta || { page: 1, pageSize: 20, total: result.data.length };
      } catch (error) {
        this.error = error instanceof Error ? error.message : String(error);
      } finally {
        this.loading = false;
      }
    },
    async createRecord(input: Partial<DomainRecord>) {
      this.loading = true;
      try {
        await createStageApproval(input);
        await this.load();
      } catch (error) {
        this.error = error instanceof Error ? error.message : String(error);
      } finally {
        this.loading = false;
      }
    },
    // Run one workbench action. Optimistic-lock version is always taken from
    // the currently loaded record so stale submissions fail on the server and
    // leave the data untouched.
    async runAction(item: DomainRecord, action: ApprovalAction, text: string) {
      this.loading = true;
      this.error = '';
      try {
        if (action.key === 'submitCorrection') {
          await submitStageApprovalCorrection(item.id, { expectedVersion: item.version, correction: text });
        } else if (action.target) {
          await transitionStageApproval(item.id, action.target, item.version, text);
        }
        await this.load();
      } catch (error) {
        this.error = error instanceof Error ? error.message : String(error);
        throw error;
      } finally {
        this.loading = false;
      }
    },
  },
});
