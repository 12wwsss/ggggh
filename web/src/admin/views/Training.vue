<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api, getUser } from '../../shared/http'

const role = (getUser() || {}).role
const statusText = (st) => ({ active: '进行中', done: '已完成', paused: '已暂停' }[st] || st)
const canCreate = computed(() => ['admin', 'safety', 'trainer', 'dept_leader'].includes(role))
const batch = ref(false)

const rows = ref([])
const loading = ref(false)
const stages = ref({})
const expanded = ref([])
const createVisible = ref(false)
const saving = ref(false)
const users = ref([])
const form = ref({ user_id: null, user_ids: [], mentor_user_id: null, title: '新员工入矿带培计划', total_weeks: 4, target: '' })
const notes = ref({})
const noteForm = ref({ kind: 'monthly', period: new Date().toISOString().slice(0, 7), content: '' })
const noteSaving = ref(false)
const mentorVisible = ref(false)
const mentorForm = ref({ plan_id: 0, user_name: '', mentor_user_id: null })

async function loadUsers() {
  if (users.value.length) return
  try {
    const data = await api.get('/api/v1/users', { page_size: 200 })
    users.value = data.list || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function removePlan(row) {
  try {
    await ElMessageBox.confirm(
      `确定删除「${row.user_name} · ${row.title}」？阶段进度和师傅小结会一起删除，不可恢复。`,
      '删除带培计划',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  try {
    await api.del(`/api/v1/training/plans/${row.id}`)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function openMentor(row) {
  await loadUsers()
  mentorForm.value = { plan_id: row.id, user_name: row.user_name, mentor_user_id: row.mentor_user_id }
  mentorVisible.value = true
}

async function saveMentor() {
  if (!mentorForm.value.mentor_user_id) return ElMessage.warning('请选择新师傅')
  try {
    await api.post(`/api/v1/training/plans/${mentorForm.value.plan_id}/mentor`, {
      mentor_user_id: mentorForm.value.mentor_user_id,
    })
    ElMessage.success('已更换师傅')
    mentorVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

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

async function openCreate() {
  createVisible.value = true
  form.value = { user_id: null, user_ids: [], mentor_user_id: null, title: '新员工入矿带培计划', total_weeks: 4, target: '' }
  batch.value = false
  if (!users.value.length) {
    try {
      const data = await api.get('/api/v1/users', { page_size: 200 })
      users.value = data.list || []
    } catch (e) {
      ElMessage.error(e.message)
    }
  }
}

async function submitCreate() {
  if (batch.value) {
    if (!form.value.user_ids.length || !form.value.mentor_user_id) {
      return ElMessage.warning('请选择员工和师傅')
    }
  } else if (!form.value.user_id || !form.value.mentor_user_id) {
    return ElMessage.warning('请选择员工和师傅')
  }
  saving.value = true
  try {
    if (batch.value) {
      const res = await api.post('/api/v1/training/plans/batch', form.value)
      ElMessage.success('已创建 ' + ((res && res.count) || 0) + ' 份计划')
    } else {
      await api.post('/api/v1/training/plans', form.value)
      ElMessage.success('已创建')
    }
    createVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function onExpand(row, expandedRows) {
  expanded.value = expandedRows
  if (!expandedRows.length) return
  try {
    const data = await api.get(`/api/v1/training/plans/${row.id}`)
    stages.value = { ...stages.value, [row.id]: data.stages || [] }
    await loadNotes(row.id)
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadNotes(planID) {
  try {
    const data = await api.get(`/api/v1/training/plans/${planID}/notes`)
    notes.value = { ...notes.value, [planID]: data.list || [] }
  } catch (e) {
    notes.value = { ...notes.value, [planID]: [] }
  }
}

async function saveNote(row) {
  const content = (noteForm.value.content || '').trim()
  if (content.length < 10) return ElMessage.warning('小结至少写 10 个字')
  noteSaving.value = true
  try {
    await api.post(`/api/v1/training/plans/${row.id}/notes`, {
      kind: noteForm.value.kind,
      period: noteForm.value.period,
      content,
    })
    ElMessage.success('已保存')
    noteForm.value.content = ''
    await loadNotes(row.id)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    noteSaving.value = false
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
      <el-button v-if="canCreate" type="primary" @click="openCreate">新建带培计划</el-button>
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

            <el-divider content-position="left">师傅小结</el-divider>
            <div class="note-form">
              <el-select v-model="noteForm.kind" size="small" style="width: 120px">
                <el-option label="月度小结" value="monthly" />
                <el-option label="期满总结" value="final" />
              </el-select>
              <el-input v-model="noteForm.period" size="small" style="width: 120px" placeholder="2026-11" />
              <el-input
                v-model="noteForm.content"
                type="textarea"
                :rows="3"
                placeholder="本月跟班情况、掌握的技能、存在的问题与下月重点（至少 10 字）"
              />
              <div style="text-align: right">
                <el-button size="small" type="primary" :loading="noteSaving" @click="saveNote(row)">
                  写小结
                </el-button>
              </div>
            </div>
            <div v-for="n in notes[row.id] || []" :key="n.id" class="note-item">
              <div class="between">
                <span class="note-meta">
                  {{ n.period }} · {{ n.kind === 'final' ? '期满总结' : '月度小结' }} · {{ n.author || '—' }}
                </span>
                <span class="muted">{{ (n.created_at || '').slice(0, 10) }}</span>
              </div>
              <div class="note-body">{{ n.content }}</div>
            </div>
            <el-empty v-if="!(notes[row.id] || []).length" description="还没有小结" :image-size="50" />
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
      <el-table-column label="状态" width="90">
        <template #default="{ row }">{{ statusText(row.status) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150">
        <template #default="{ row }">
          <el-button link type="primary" @click="openMentor(row)">换师傅</el-button>
          <el-button v-if="role === 'admin'" link type="danger" @click="removePlan(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="createVisible" title="新建带培计划" width="520px">
      <el-form label-width="90px">
        <el-form-item label="方式">
          <el-switch v-model="batch" active-text="批量给多人建计划" />
        </el-form-item>
        <el-form-item v-if="!batch" label="员工">
          <el-select v-model="form.user_id" filterable placeholder="选择员工" style="width: 100%">
            <el-option
              v-for="u in users"
              :key="u.id"
              :label="u.name + '（' + u.username + '）'"
              :value="u.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-else label="员工（多选）">
          <el-select v-model="form.user_ids" multiple filterable placeholder="选择一名或多名新员工" style="width: 100%">
            <el-option
              v-for="u in users"
              :key="u.id"
              :label="u.name + '（' + u.username + '）'"
              :value="u.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="师傅">
          <el-select v-model="form.mentor_user_id" filterable placeholder="选择师傅" style="width: 100%">
            <el-option
              v-for="u in users"
              :key="u.id"
              :label="u.name + '（' + u.username + '）'"
              :value="u.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="计划名称">
          <el-input v-model="form.title" />
        </el-form-item>
        <el-form-item label="周期（周）">
          <el-input-number v-model="form.total_weeks" :min="1" :max="12" />
        </el-form-item>
        <el-form-item label="目标">
          <el-input v-model="form.target" placeholder="默认：完成入矿教育、岗位跟班与独立上岗考核" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitCreate">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="mentorVisible" title="更换带培师傅" width="460px">
      <el-form label-width="80px">
        <el-form-item label="员工">{{ mentorForm.user_name }}</el-form-item>
        <el-form-item label="新师傅">
          <el-select v-model="mentorForm.mentor_user_id" filterable style="width: 100%" placeholder="选择师傅">
            <el-option v-for="u in users" :key="u.id" :label="u.name + '（' + u.username + '）'" :value="u.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="mentorVisible = false">取消</el-button>
        <el-button type="primary" @click="saveMentor">保存</el-button>
      </template>
    </el-dialog>
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
.note-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: #fafaf7;
  border: 1px dashed #e2e0d6;
  border-radius: 6px;
  padding: 10px 12px;
}
.note-item {
  padding: 10px 2px;
  border-bottom: 1px solid #f0f2f5;
}
.note-item:last-child {
  border-bottom: none;
}
.note-meta {
  font-size: 13px;
  color: #59636e;
  font-weight: 600;
}
.note-body {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.8;
  color: #2f3438;
  white-space: pre-wrap;
}
</style>
