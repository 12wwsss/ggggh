<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../../shared/http'

const rows = ref([])
const loading = ref(false)
const stages = ref({})
const expanded = ref([])

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/training/plans', { page_size: 50 })
    rows.value = data.list || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function onExpand(row, expandedRows) {
  expanded.value = expandedRows
  if (!expandedRows.length) return
  try {
    const data = await api.get(`/api/v1/training/plans/${row.id}`)
    stages.value = { ...stages.value, [row.id]: data.stages || [] }
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function toggle(row, st) {
  try {
    await api.post(`/api/v1/training/stages/${st.id}`, { done: !st.done })
    const data = await api.get(`/api/v1/training/plans/${row.id}`)
    stages.value = { ...stages.value, [row.id]: data.stages || [] }
    row.stage_done = data.stage_done
    row.stage_total = data.stage_total
    ElMessage.success(!st.done ? '已标记完成' : '已取消完成')
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div class="muted">新员工带培计划：勾选阶段即完成，员工在小程序「新员工带培」实时看到进度。</div>
    </div>

    <el-table :data="rows" v-loading="loading" row-key="id" @expand-change="onExpand">
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="stages">
            <div v-for="st in stages[row.id] || []" :key="st.id" class="stage">
              <el-checkbox :model-value="st.done" @change="toggle(row, st)" />
              <div class="grow">
                <div class="goal">{{ st.week }} · {{ st.goal }}</div>
                <div class="muted">{{ (st.tasks || []).join('；') }}</div>
              </div>
            </div>
            <el-empty v-if="!(stages[row.id] || []).length" description="没有阶段" :image-size="60" />
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="user_name" label="员工" width="120" />
      <el-table-column prop="dept_name" label="区队" width="120" />
      <el-table-column prop="title" label="计划" min-width="200" />
      <el-table-column prop="mentor" label="师傅" width="100" />
      <el-table-column label="周期" width="90">
        <template #default="{ row }">{{ row.total_weeks }} 周</template>
      </el-table-column>
      <el-table-column label="进度" min-width="180">
        <template #default="{ row }">
          <el-progress
            :percentage="row.stage_total ? Math.round((row.stage_done / row.stage_total) * 100) : 0"
            :stroke-width="8"
          />
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="90" />
    </el-table>
  </div>
</template>

<style scoped>
.toolbar {
  margin-bottom: 12px;
}
.stages {
  padding: 4px 8px;
}
.stage {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 10px 0;
  border-bottom: 1px solid #f0f2f5;
}
.stage:last-child {
  border-bottom: none;
}
.goal {
  font-weight: 600;
}
.grow {
  flex: 1;
}
</style>
