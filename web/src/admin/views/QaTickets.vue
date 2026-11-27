<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../../shared/http'

const rows = ref([])
const total = ref(0)
const pending = ref(0)
const loading = ref(false)
const status = ref('pending')
const keyword = ref('')
const page = reactive({ page: 1, page_size: 20 })

const dialog = ref(false)
const current = ref(null)
const reply = ref('')
const saving = ref(false)

const statusText = (s) => ({ pending: '待回复', replied: '已回复', closed: '已关闭' }[s] || s)
const statusType = (s) => ({ pending: 'warning', replied: 'success', closed: 'info' }[s] || 'info')

async function load() {
  loading.value = true
  try {
    const params = { ...page }
    if (status.value) params.status = status.value
    if (keyword.value.trim()) params.keyword = keyword.value.trim()
    const data = await api.get('/api/v1/qa/tickets', params)
    rows.value = data.list || []
    total.value = data.total || 0
    pending.value = data.pending || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function switchStatus(s) {
  status.value = s
  page.page = 1
  load()
}

onMounted(load)

function openReply(row) {
  current.value = row
  reply.value = row.reply || ''
  dialog.value = true
}

async function submitReply() {
  const text = reply.value.trim()
  if (!text) return ElMessage.warning('回复内容不能为空')
  saving.value = true
  try {
    await api.post(`/api/v1/qa/tickets/${current.value.id}/reply`, { reply: text })
    ElMessage.success('已回复，员工会在消息中心看到')
    dialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function closeTicket(row) {
  try {
    await api.post(`/api/v1/qa/tickets/${row.id}/close`, {})
    ElMessage.success('已关闭')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

const emptyText = computed(() => (status.value === 'pending' ? '没有待回复的转人工' : '没有记录'))
</script>

<template>
  <div class="panel">
    <div style="display: flex; gap: 10px; align-items: center; margin-bottom: 14px">
      <el-radio-group :model-value="status" @update:model-value="switchStatus">
        <el-radio-button value="pending">待回复（{{ pending }}）</el-radio-button>
        <el-radio-button value="replied">已回复</el-radio-button>
        <el-radio-button value="closed">已关闭</el-radio-button>
        <el-radio-button value="">全部</el-radio-button>
      </el-radio-group>
      <el-input
        v-model="keyword"
        placeholder="问题或员工姓名"
        clearable
        style="width: 220px"
        @change="(page.page = 1), load()"
      />
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" :empty-text="emptyText">
      <el-table-column label="时间" width="140">
        <template #default="{ row }">{{ (row.created_at || '').slice(0, 16).replace('T', ' ') }}</template>
      </el-table-column>
      <el-table-column prop="user_name" label="员工" width="100" />
      <el-table-column prop="dept_name" label="区队" width="110" />
      <el-table-column prop="question" label="问题" min-width="240" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="assignee" label="回复人" width="100" />
      <el-table-column label="操作" width="150">
        <template #default="{ row }">
          <el-button link type="primary" @click="openReply(row)">
            {{ row.status === 'pending' ? '回复' : '查看/补充' }}
          </el-button>
          <el-button v-if="row.status !== 'closed'" link type="danger" @click="closeTicket(row)">关闭</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top: 14px; text-align: right">
      <el-pagination
        v-model:current-page="page.page"
        v-model:page-size="page.page_size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>

    <el-dialog v-model="dialog" title="回复转人工" width="620px">
      <template v-if="current">
        <div class="q">{{ current.question }}</div>
        <div v-if="current.answer" class="a">AI 原回答：{{ current.answer }}</div>
        <el-input v-model="reply" type="textarea" :rows="5" placeholder="写清楚依据（规程条款）、处置建议，员工会在消息中心看到" />
      </template>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitReply">发送回复</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.panel {
  background: #fff;
  border-radius: 10px;
  padding: 18px;
}
.q {
  font-size: 15px;
  font-weight: 600;
  line-height: 1.8;
  color: #10151a;
  margin-bottom: 8px;
}
.a {
  font-size: 13px;
  color: #6b7684;
  background: #fafaf7;
  border-radius: 6px;
  padding: 10px 12px;
  margin-bottom: 14px;
  line-height: 1.8;
}
</style>
