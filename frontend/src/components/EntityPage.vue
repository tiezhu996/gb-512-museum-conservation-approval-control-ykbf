<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import type { ApprovalAction, DomainRecord, EntityConfig } from '../types/domain';
import { formatDate, nextStatus } from '../utils/format';
import { useAuth } from '../hooks/useAuth';
import StatusBadge from './common/StatusBadge.vue';
import MetricCard from './common/MetricCard.vue';
import ConfirmDialog from './common/ConfirmDialog.vue';
import EmptyState from './common/EmptyState.vue';

interface PendingOperation { item: DomainRecord; action: ApprovalAction }

const props = defineProps<{ config: EntityConfig; store: any }>();
const { session } = useAuth();
const search = ref('');
const showCreate = ref(false);
const pending = ref<PendingOperation | null>(null);
const opinion = ref('');
const opinionError = ref('');
const roleRank: Record<string, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };
const canWrite = computed(() => (roleRank[session.value?.role || ''] || 0) >= roleRank.operator);
const isApproval = computed(() => props.config.path === 'approvals');
const highRisk = computed(() => props.store.items.filter((item: DomainRecord) => ['high', 'critical'].includes(item.riskLevel)).length);

onMounted(() => void props.store.load(props.config.path));

// 阶段审批：返回当前角色在该状态下允许执行的操作；其它实体沿用线性推进。
function availableActions(item: DomainRecord): ApprovalAction[] {
  if (!isApproval.value) return [];
  return props.config.actions?.[item.status] ?? [];
}

function canRun(action: ApprovalAction): boolean {
  if (!session.value) return false;
  return roleRank[session.value.role] >= (roleRank[action.minRole] ?? 0);
}

// 非阶段审批实体保持原有“推进到下一状态”的单按钮逻辑。
function legacyTarget(item: DomainRecord): string | null {
  if (isApproval.value) return null;
  const target = nextStatus(item.status, props.config.statuses);
  if (!target || !canWrite.value) return null;
  return target;
}

function canTransition(item: DomainRecord): boolean {
  return legacyTarget(item) !== null;
}

function openOperation(item: DomainRecord, action: ApprovalAction) {
  if (!canRun(action)) return;
  opinion.value = '';
  opinionError.value = '';
  pending.value = { item, action };
}

// 审批操作都必须携带意见；其它实体保留默认说明。
async function confirmTransition() {
  if (!pending.value) return;
  const { item, action } = pending.value;
  const note = opinion.value.trim();
  if (isApproval.value) {
    if (!canRun(action) || note.length < 3) {
      opinionError.value = '请填写不少于 3 个字符的意见后再提交';
      return;
    }
  }
  if (!isApproval.value && legacyTarget(item) === null) return;
  await props.store.transition(props.config.path, item, action.target, isApproval.value ? note : '前端工作台人工确认');
  pending.value = null;
}

function createDemo() {
  if (!canWrite.value) return;
  const now = Date.now();
  void props.store.createRecord(props.config.path, {
    code: `${props.config.key.toUpperCase()}-${String(now).slice(-6)}`,
    name: `新增${props.config.label}`,
    description: '通过前端工作台创建的业务记录',
    facility: '保护实验室', owner: '保护操作员', category: '阶段复核', riskLevel: 'medium',
    metricValue: 25, metricUnit: 'score', effectiveAt: new Date().toISOString(),
    evidence: '已核对材料检测与影像证据', relatedCode: '',
  });
  showCreate.value = false;
}

// 没有可执行操作时给出当前状态对应的说明。
function idleHint(item: DomainRecord): string {
  if (!canWrite.value) return '只读权限';
  if (!isApproval.value) return '流程结束';
  if (availableActions(item).some(canRun)) return '';
  switch (item.status) {
    case 'review': return '等待复核员审批';
    case 'pending_correction': return '等待操作员补正';
    case 'approved': return '审批已通过';
    case 'rejected': return '审批已驳回';
    default: return '流程结束';
  }
}
</script>

<template>
  <main class="workspace">
    <header class="page-header">
      <div><p class="eyebrow">业务工作台</p><h1>{{ config.label }}</h1><p>统一管理{{ config.label }}的状态、风险、证据与责任人。</p></div>
      <el-button v-if="canWrite" type="primary" @click="showCreate = true">新增{{ config.label }}</el-button>
    </header>
    <section class="metrics">
      <MetricCard label="记录总数" :value="store.meta.total" detail="当前筛选范围"/>
      <MetricCard label="高风险" :value="highRisk" detail="需要优先复核"/>
      <MetricCard label="状态种类" :value="new Set(store.items.map((item: DomainRecord) => item.status)).size" detail="状态机覆盖"/>
    </section>
    <slot name="insight" :items="store.items"/>
    <section class="toolbar">
      <el-input v-model="search" :placeholder="`搜索${config.label}编码或名称`" clearable/>
      <el-button type="primary" @click="store.load(config.path, search)">查询</el-button>
      <el-button @click="search = ''; store.load(config.path)">重置</el-button>
    </section>
    <el-alert v-if="store.error" :title="store.error" type="error" show-icon/>
    <EmptyState v-if="!store.loading && store.items.length === 0" :title="`暂无${config.label}`" description="调整查询条件或创建第一条记录。"/>
    <section v-else class="table-shell">
      <el-table v-loading="store.loading" :data="store.items">
        <el-table-column prop="code" label="编码" width="150"/>
        <el-table-column label="名称" min-width="180"><template #default="{ row }"><strong>{{ row.name }}</strong><small>{{ row.facility }}</small></template></el-table-column>
        <el-table-column label="状态" width="150"><template #default="{ row }"><StatusBadge :status="row.status"/></template></el-table-column>
        <el-table-column prop="riskLevel" label="风险" width="90"/>
        <el-table-column prop="owner" label="责任人"/>
        <el-table-column label="指标"><template #default="{ row }">{{ row.metricValue }} {{ row.metricUnit }}</template></el-table-column>
        <el-table-column label="复核批次" width="100">
          <template #default="{ row }">
            <span v-if="isApproval && row.opinions?.length">第 {{ row.opinions[row.opinions.length - 1].batch }} 批</span>
            <span v-else-if="isApproval" class="muted">未提交</span>
            <span v-else class="muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180"><template #default="{ row }">{{ formatDate(row.updatedAt) }}</template></el-table-column>
        <el-table-column label="操作" :width="isApproval ? 250 : 200">
          <template #default="{ row }">
            <template v-if="isApproval">
              <el-button v-for="action in availableActions(row).filter(canRun)" :key="action.target"
                link :type="action.tone" @click="openOperation(row, action)">{{ action.label }}</el-button>
              <span v-if="!availableActions(row).some(canRun)" class="muted">{{ idleHint(row) }}</span>
            </template>
            <template v-else>
              <el-button v-if="canTransition(row)" link type="primary"
                @click="openOperation(row, { target: legacyTarget(row)!, label: '推进状态', minRole: 'operator', tone: 'primary', placeholder: '' })">
                推进至 {{ legacyTarget(row) }}
              </el-button>
              <span v-else class="muted">流程结束</span>
            </template>
          </template>
        </el-table-column>
      </el-table>
    </section>
    <ConfirmDialog v-model="showCreate" :title="`新增${config.label}`" @confirm="createDemo"><p>将创建一条包含责任人、风险和保护证据信息的记录。</p></ConfirmDialog>
    <ConfirmDialog :model-value="Boolean(pending)" :title="`确认${pending?.action.label ?? '状态迁移'}`"
      @update:model-value="pending = null" @confirm="confirmTransition">
      <p>本次意见会作为独立版本写入审批历史，并记录操作者与请求 ID，历史意见不可改写。</p>
      <p v-if="isApproval"><strong>{{ pending?.item.status }} → {{ pending?.action.target }}</strong></p>
      <p v-else><strong>{{ pending?.item.status }} → {{ pending?.action.target }}</strong></p>
      <el-input
        v-if="isApproval"
        v-model="opinion"
        type="textarea"
        :rows="3"
        :placeholder="pending?.action.placeholder"
        maxlength="500"
        show-word-limit
        class="opinion-input"
      />
      <small v-if="opinionError" class="opinion-error">{{ opinionError }}</small>
    </ConfirmDialog>
  </main>
</template>
