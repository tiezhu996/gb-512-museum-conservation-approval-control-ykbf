<script setup lang="ts">
import type { DomainRecord } from '../../types/domain';
import StatusBadge from './StatusBadge.vue';
import EmptyState from './EmptyState.vue';
import { currentBatch, OPINION_KIND_LABELS } from '../../utils/approval-actions';
import { APPROVAL_STATUS_LABELS } from '../../types/status';

defineProps<{ records: DomainRecord[] }>();

function latestOpinion(item: DomainRecord) {
  return item.opinions && item.opinions.length ? item.opinions[item.opinions.length - 1] : null;
}

function statusLabel(status: string): string {
  return APPROVAL_STATUS_LABELS[status] || status;
}
</script>

<template>
  <section class="approval-timeline" aria-label="阶段审批时间线">
    <header><strong>阶段审批时间线</strong><span>意见按版本追加，历史不可覆盖；退回补正开启新复核批次</span></header>
    <div v-if="records.length" class="timeline-grid">
      <article v-for="item in records.slice(0, 4)" :key="item.id">
        <div class="timeline-marker">v{{ item.version }} · 批次{{ currentBatch(item.opinions) }}</div>
        <div>
          <strong>{{ item.code }} · {{ item.name }}</strong>
          <p v-if="latestOpinion(item)">
            <el-tag size="small" effect="plain">{{ OPINION_KIND_LABELS[latestOpinion(item)!.kind] || latestOpinion(item)!.kind }}</el-tag>
            {{ latestOpinion(item)!.actor }}：{{ latestOpinion(item)!.opinion }}
          </p>
          <p v-else>等待首条阶段意见</p>
        </div>
        <StatusBadge :status="statusLabel(item.status)"/>
      </article>
    </div>
    <EmptyState v-else title="暂无审批轨迹" description="方案提交评审后，版本意见会在这里显示。"/>
  </section>
</template>
