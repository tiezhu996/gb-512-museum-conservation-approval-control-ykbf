<script setup lang="ts">
import { computed } from 'vue';
import type { ApprovalOpinion, DomainRecord } from '../../types/domain';
import { formatDate } from '../../utils/format';
import StatusBadge from './StatusBadge.vue';
import EmptyState from './EmptyState.vue';

const props = defineProps<{ records: DomainRecord[] }>();

// 工作台展示完整批次：阶段审批按复核批次分组，同一批次内的提交、退回补正、
// 通过/驳回意见都保留；历史意见只追加不改写。
const recordsWithOpinions = computed(() =>
  props.records
    .filter((item) => item.opinions && item.opinions.length > 0)
    .slice(0, 4)
    .map((item) => {
      const byBatch = new Map<number, ApprovalOpinion[]>();
      for (const opinion of item.opinions ?? []) {
        const group = byBatch.get(opinion.batch) ?? [];
        group.push(opinion);
        byBatch.set(opinion.batch, group);
      }
      const batches = [...byBatch.entries()]
        .sort((a, b) => a[0] - b[0])
        .map(([batch, opinions]) => ({ batch, opinions: [...opinions].sort((a, b) => a.version - b.version) }));
      return { item, batches };
    }),
);
</script>

<template>
  <section class="approval-timeline" aria-label="阶段审批时间线">
    <header><strong>阶段审批时间线</strong><span>意见按复核批次追加，历史不可覆盖</span></header>
    <div v-if="recordsWithOpinions.length" class="timeline-grid">
      <article v-for="entry in recordsWithOpinions" :key="entry.item.id">
        <div class="timeline-head">
          <div class="timeline-marker">v{{ entry.item.version }}</div>
          <div class="timeline-title">
            <strong>{{ entry.item.code }} · {{ entry.item.name }}</strong>
            <small>责任人：{{ entry.item.owner }} · 共 {{ entry.batches.length }} 个复核批次</small>
          </div>
          <StatusBadge :status="entry.item.status"/>
        </div>
        <ol class="batch-list">
          <li v-for="batch in entry.batches" :key="batch.batch" class="batch-group">
            <header>复核批次 {{ batch.batch }}</header>
            <ul>
              <li v-for="opinion in batch.opinions" :key="opinion.id">
                <StatusBadge :status="opinion.status"/>
                <p>{{ opinion.opinion }}</p>
                <small>{{ opinion.actor }} · {{ formatDate(opinion.createdAt) }} · 请求 {{ opinion.requestId }}</small>
              </li>
            </ul>
          </li>
        </ol>
      </article>
    </div>
    <EmptyState v-else title="暂无审批轨迹" description="方案提交评审后，版本意见会在这里显示。"/>
  </section>
</template>
