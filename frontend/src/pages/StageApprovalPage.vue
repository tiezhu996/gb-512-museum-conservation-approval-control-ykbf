<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import MetricCard from '../components/common/MetricCard.vue';
import StatusBadge from '../components/common/StatusBadge.vue';
import ApprovalTimeline from '../components/common/ApprovalTimeline.vue';
import EmptyState from '../components/common/EmptyState.vue';
import { useAuth } from '../hooks/useAuth';
import { useStageApprovalStore } from '../stores/stage-approval';
import { ALL_APPROVAL_STATE, APPROVAL_STATUS_LABELS } from '../types/status';
import type { ApprovalAction, ApprovalOpinion, DomainRecord } from '../types/domain';
import { actionPrompt, availableActions, currentBatch, groupByBatch, OPINION_KIND_LABELS } from '../utils/approval-actions';
import { formatDate } from '../utils/format';

const store = useStageApprovalStore();
const { session } = useAuth();

const search = ref('');
const statusFilter = ref('');
const showCreate = ref(false);

// Pending action dialog state.
const pending = ref<{ item: DomainRecord; action: ApprovalAction } | null>(null);
const opinionText = ref('');
const submitting = ref(false);

// Detail drawer showing the full immutable batch/opinion history.
const detail = ref<DomainRecord | null>(null);

const role = computed(() => session.value?.role || 'viewer');
const pendingCorrectionCount = computed(() => store.items.filter((item) => item.status === 'correction').length);
const inReviewCount = computed(() => store.items.filter((item) => item.status === 'review').length);

const statusOptions = ALL_APPROVAL_STATE.map((value) => ({ value, label: APPROVAL_STATUS_LABELS[value] }));

onMounted(() => void store.load());

function actionsFor(item: DomainRecord): ApprovalAction[] {
  return availableActions(item, role.value);
}

function statusLabel(status: string): string {
  return APPROVAL_STATUS_LABELS[status] || status;
}

function openAction(item: DomainRecord, action: ApprovalAction) {
  pending.value = { item, action };
  opinionText.value = '';
}

async function confirmAction() {
  if (!pending.value) return;
  const text = opinionText.value.trim();
  if (text.length < pending.value.action.minLength) {
    ElMessage.warning(`${actionPrompt(pending.value.action)}至少 ${pending.value.action.minLength} 个字符`);
    return;
  }
  submitting.value = true;
  try {
    await store.runAction(pending.value.item, pending.value.action, text);
    ElMessage.success('操作已记录为独立版本');
    pending.value = null;
  } catch {
    // store.error already surfaces the reason (permission / stale version).
  } finally {
    submitting.value = false;
  }
}

async function createDemo() {
  const now = Date.now();
  await store.createRecord({
    code: `SA-${String(now).slice(-6)}`,
    name: '新增阶段审批',
    description: '通过前端工作台创建的业务记录',
    facility: '保护实验室', owner: '保护操作员', category: '阶段复核', riskLevel: 'medium',
    metricValue: 25, metricUnit: 'score', effectiveAt: new Date().toISOString(),
    evidence: '已核对材料检测与影像证据', relatedCode: '',
  });
  showCreate.value = false;
}

function openDetail(item: DomainRecord) {
  detail.value = item;
}

const detailBatches = computed(() => groupByBatch(detail.value?.opinions));

function opinionKindLabel(opinion: ApprovalOpinion): string {
  return OPINION_KIND_LABELS[opinion.kind] || opinion.kind;
}

function toneFor(opinion: ApprovalOpinion): 'success' | 'danger' | 'warning' | 'info' {
  if (opinion.status === 'approved') return 'success';
  if (opinion.status === 'rejected') return 'danger';
  if (opinion.status === 'correction') return 'warning';
  return 'info';
}

function actionType(action: ApprovalAction): 'primary' | 'danger' | 'warning' | 'success' {
  if (action.danger) return 'danger';
  if (action.warning) return 'warning';
  if (action.key === 'approve') return 'success';
  return 'primary';
}
</script>

<template>
  <main class="workspace">
    <header class="page-header">
      <div>
        <p class="eyebrow">业务工作台</p>
        <h1>阶段审批</h1>
        <p>复核人可通过、驳回或退回补正；操作员对待补正记录提交补正说明后开启新复核批次，历史意见全程留痕。</p>
      </div>
      <el-button v-if="role !== 'viewer'" type="primary" @click="showCreate = true">新增阶段审批</el-button>
    </header>

    <section class="metrics">
      <MetricCard label="记录总数" :value="store.meta.total" detail="当前筛选范围"/>
      <MetricCard label="待复核" :value="inReviewCount" detail="等待复核员决定"/>
      <MetricCard label="待补正" :value="pendingCorrectionCount" detail="等待操作员提交补正说明"/>
      <MetricCard label="当前角色" :value="role" detail="仅显示该角色可执行的操作"/>
    </section>

    <ApprovalTimeline :records="store.items"/>

    <section class="toolbar">
      <el-input v-model="search" placeholder="搜索阶段审批编码或名称" clearable style="max-width: 280px"/>
      <el-select v-model="statusFilter" placeholder="全部状态" clearable style="width: 150px">
        <el-option v-for="option in statusOptions" :key="option.value" :value="option.value" :label="option.label"/>
      </el-select>
      <el-button type="primary" @click="store.load(search, statusFilter)">查询</el-button>
      <el-button @click="search = ''; statusFilter = ''; store.load('')">重置</el-button>
    </section>

    <el-alert v-if="store.error" :title="store.error" type="error" show-icon :closable="false"/>

    <EmptyState v-if="!store.loading && store.items.length === 0" title="暂无阶段审批" description="调整查询条件或创建第一条记录。"/>
    <section v-else class="table-shell">
      <el-table v-loading="store.loading" :data="store.items">
        <el-table-column prop="code" label="编码" width="130">
          <template #default="{ row }"><el-link type="primary" @click="openDetail(row)">{{ row.code }}</el-link></template>
        </el-table-column>
        <el-table-column label="名称" min-width="170">
          <template #default="{ row }"><strong>{{ row.name }}</strong><small>{{ row.facility }}</small></template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }"><StatusBadge :status="statusLabel(row.status)"/></template>
        </el-table-column>
        <el-table-column label="复核批次" width="100">
          <template #default="{ row }">第 {{ currentBatch(row.opinions) }} 批 · v{{ row.version }}</template>
        </el-table-column>
        <el-table-column prop="owner" label="责任人" width="110"/>
        <el-table-column label="最近意见" min-width="220">
          <template #default="{ row }">
            <span v-if="row.opinions && row.opinions.length">
              <el-tag size="small" effect="plain" :type="toneFor(row.opinions[row.opinions.length - 1])">
                {{ opinionKindLabel(row.opinions[row.opinions.length - 1]) }}
              </el-tag>
              {{ row.opinions[row.opinions.length - 1].actor }}：{{ row.opinions[row.opinions.length - 1].opinion }}
            </span>
            <span v-else class="muted">尚无意见</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="170">
          <template #default="{ row }">{{ formatDate(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <template v-if="actionsFor(row).length">
              <el-button
                v-for="action in actionsFor(row)"
                :key="action.key"
                link
                :type="actionType(action)"
                @click="openAction(row, action)"
              >{{ action.label }}</el-button>
            </template>
            <el-button link type="info" @click="openDetail(row)">批次详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <!-- Create -->
    <el-dialog v-model="showCreate" title="新增阶段审批" width="440px">
      <p>将创建一条草稿状态、责任人与证据信息齐全的阶段审批，提交后由操作员发起复核。</p>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" @click="createDemo">确认</el-button>
      </template>
    </el-dialog>

    <!-- Action with written opinion / correction explanation -->
    <el-dialog
      :model-value="Boolean(pending)"
      :title="pending ? `${pending.item.code} · ${pending.action.label}` : ''"
      width="480px"
      @close="pending = null"
    >
      <p v-if="pending" class="muted">
        <StatusBadge :status="statusLabel(pending.item.status)"/>
        → {{ pending.action.target ? statusLabel(pending.action.target) : '开启新复核批次' }}
        （第 {{ currentBatch(pending.item.opinions) + (pending.action.key === 'submitCorrection' ? 1 : 0) }} 批）
      </p>
      <el-input
        v-model="opinionText"
        :placeholder="`${actionPrompt(pending?.action || ({} as ApprovalAction))}（必填，历史不可改写）`"
        type="textarea"
        :rows="4"
        maxlength="1000"
        show-word-limit
      />
      <template #footer>
        <el-button @click="pending = null">取消</el-button>
        <el-button :type="pending ? actionType(pending.action) : 'primary'" :loading="submitting" @click="confirmAction">确认提交</el-button>
      </template>
    </el-dialog>

    <!-- Full batch history drawer -->
    <el-drawer v-model="detail" :title="detail ? `${detail.code} · 完整复核批次` : ''" size="520px">
      <div v-if="detail" class="detail-body">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="名称">{{ detail.name }}</el-descriptions-item>
          <el-descriptions-item label="当前状态"><StatusBadge :status="statusLabel(detail.status)"/></el-descriptions-item>
          <el-descriptions-item label="责任人">{{ detail.owner }}</el-descriptions-item>
          <el-descriptions-item label="复核批次">第 {{ currentBatch(detail.opinions) }} 批 · 版本 v{{ detail.version }}</el-descriptions-item>
          <el-descriptions-item label="设施">{{ detail.facility }}</el-descriptions-item>
          <el-descriptions-item label="关联编码">{{ detail.relatedCode || '-' }}</el-descriptions-item>
        </el-descriptions>

        <h3>批次与意见（不可改写）</h3>
        <el-timeline v-if="detailBatches.length">
          <el-timeline-item
            v-for="group in detailBatches"
            :key="group.batch"
            :timestamp="`第 ${group.batch} 复核批次`"
            placement="top"
            type="primary"
          >
            <el-card v-for="opinion in group.opinions" :key="opinion.id" shadow="never" class="opinion-card">
              <div class="opinion-head">
                <el-tag size="small" :type="toneFor(opinion)">{{ opinionKindLabel(opinion) }}</el-tag>
                <strong>v{{ opinion.version }} · {{ opinion.actor }}（{{ opinion.role }}）</strong>
                <StatusBadge :status="statusLabel(opinion.status)"/>
              </div>
              <p class="opinion-text">{{ opinion.opinion }}</p>
              <small class="muted">{{ formatDate(opinion.createdAt) }} · 请求 ID：{{ opinion.requestId }}</small>
            </el-card>
          </el-timeline-item>
        </el-timeline>
        <EmptyState v-else title="暂无意见" description="该审批尚未提交复核。"/>

        <div v-if="actionsFor(detail).length" class="detail-actions">
          <el-button
            v-for="action in actionsFor(detail)"
            :key="action.key"
            :type="actionType(action)"
            @click="openAction(detail, action)"
          >{{ action.label }}</el-button>
        </div>
      </div>
    </el-drawer>
  </main>
</template>

<style scoped>
.detail-body { padding: 0 4px; }
.detail-body h3 { margin: 20px 0 12px; }
.opinion-card { margin-bottom: 10px; }
.opinion-head { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; flex-wrap: wrap; }
.opinion-text { margin: 6px 0; white-space: pre-wrap; }
.detail-actions { margin-top: 16px; display: flex; gap: 8px; flex-wrap: wrap; }
</style>
