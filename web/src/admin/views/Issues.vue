<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api, download, getUser } from '../../shared/http'
import { fmtTime, issueStatus, severityText } from '../../shared/format'
import SealStamp from '../components/SealStamp.vue'

const rows = ref([])
const total = ref(0)
const loading = ref(false)
const sealedId = ref(0)
const query = reactive({ page: 1, page_size: 20, status: '', keyword: '' })
const stats = ref({})

const drawer = ref(false)
const current = ref(null)
const flow = ref([])
const attachments = ref([])

const openCount = computed(() => {
  const s = stats.value || {}
  if (s.open !== undefined && s.open !== null) return s.open
  return (s.reported || 0) + (s.accepted || 0) + (s.rectified || 0)
})

// 与后端状态机保持一致：只有还能往下走的单据才叫“处理”，
// 已闭环 / 已驳回的只能看，按钮文案改成“查看”。
function canAct(row) {
  const role = getUser()?.role
  switch (row.status) {
    case 'reported':
      return role === 'admin' || role === 'dept_leader'
    case 'accepted':
      return true
    case 'rectified':
      return role === 'admin' || role === 'dept_leader'
    case 'verified':
      return role === 'admin'
    default:
      return false
  }
}

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/issues', { ...query })
    rows.value = data.list || []
    total.value = data.total || 0
    stats.value = await api.get('/api/v1/issues/stats')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function open(row) {
  current.value = row
  drawer.value = true
  const [f, a] = await Promise.all([
    api.get(`/api/v1/issues/${row.id}/flow`),
    api.get(`/api/v1/issues/${row.id}/attachments`),
  ])
  flow.value = f || []
  attachments.value = a || []
}

async function downloadAttachment(a) {
  try {
    await download(`/api/v1/issues/${current.value.id}/attachments/${a.id}`, a.file_name.split(/[\\/]/).pop())
  } catch (e) {
    ElMessage.error(`下载失败：${e.message}`)
  }
}

async function act(kind) {
  const id = current.value.id
  try {
    if (kind === 'accept') {
      const { value } = await ElMessageBox.prompt('受理意见（可留空）', '受理')
      await api.post(`/api/v1/issues/${id}/accept`, { comment: value || '' })
    } else if (kind === 'reject') {
      const { value } = await ElMessageBox.prompt('驳回原因', '驳回', { inputValidator: (v) => Boolean(v) || '必填' })
      await api.post(`/api/v1/issues/${id}/reject`, { comment: value })
    } else if (kind === 'rectify') {
      const { value } = await ElMessageBox.prompt('整改结果', '提交整改', { inputValidator: (v) => Boolean(v) || '必填' })
      await api.post(`/api/v1/issues/${id}/rectify`, { result: value })
    } else if (kind === 'verify') {
      const { value } = await ElMessageBox.prompt('验收意见', '验收通过', { inputValue: '现场验收合格' })
      await api.post(`/api/v1/issues/${id}/verify`, { pass: true, comment: value || '现场验收合格' })
    } else if (kind === 'verify_reject') {
      const { value } = await ElMessageBox.prompt('退回原因', '验收不合格', { inputValidator: (v) => Boolean(v) || '必填' })
      await api.post(`/api/v1/issues/${id}/verify`, { pass: false, comment: value })
    } else if (kind === 'close') {
      await api.post(`/api/v1/issues/${id}/close`, { comment: '闭环' })
      // 闭环后盖一枚章，提示这次状态变化
      sealedId.value = id
      setTimeout(() => {
        if (sealedId.value === id) sealedId.value = 0
      }, 1600)
    }
    ElMessage.success('操作成功')
    await load()
    const fresh = await api.get(`/api/v1/issues/${id}`)
    current.value = fresh
    flow.value = await api.get(`/api/v1/issues/${id}/flow`)
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}
</script>

<template>
  <div class="panel">
    <div style="display: flex; gap: 10px; margin-bottom: 14px; flex-wrap: wrap">
      <el-input v-model="query.keyword" placeholder="标题或编号" clearable style="width: 200px" @change="load" />
      <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 150px" @change="load">
        <el-option v-for="(v, k) in issueStatus" :key="k" :label="v.text" :value="k" />
      </el-select>
      <el-tag :type="openCount ? 'warning' : 'info'">未闭环 {{ openCount }}</el-tag>
      <el-tag :type="stats.overdue ? 'danger' : 'info'">超时 {{ stats.overdue ?? 0 }}</el-tag>
    </div>

    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="code" label="编号" width="150" />
      <el-table-column prop="title" label="问题" min-width="240" show-overflow-tooltip />
      <el-table-column prop="dept_name" label="责任区队" width="120" />
      <el-table-column label="等级" width="80">
        <template #default="{ row }">{{ severityText[row.severity] }}</template>
      </el-table-column>
      <el-table-column label="状态" width="96">
        <template #default="{ row }">
          <SealStamp v-if="row.status === 'closed'" text="已闭环" :animate="sealedId === row.id" />
          <el-tag v-else :type="issueStatus[row.status]?.type || 'info'" size="small">
            {{ issueStatus[row.status]?.text }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="超时" width="70">
        <template #default="{ row }">
          <el-tag v-if="row.overdue" type="danger" size="small">是</el-tag>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="上报时间" width="150">
        <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="80">
        <template #default="{ row }">
          <el-button link type="primary" @click="open(row)">{{ canAct(row) ? '处理' : '查看' }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top: 14px; display: flex; justify-content: flex-end">
      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.page_size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>

    <el-drawer v-model="drawer" :title="current?.code || '问题详情'" size="560px">
      <template v-if="current">
        <h3 style="margin: 0 0 8px">{{ current.title }}</h3>
        <div class="muted" style="margin-bottom: 12px">
          {{ current.dept_name || '—' }} · {{ severityText[current.severity] }} · {{ issueStatus[current.status]?.text }}
        </div>
        <p style="white-space: pre-wrap; line-height: 1.8">{{ current.description }}</p>
        <div class="muted">上报人：{{ current.reporter }} · {{ fmtTime(current.created_at) }}</div>
        <div class="muted">整改结果：{{ current.result || '—' }}</div>
        <div class="muted">验收意见：{{ current.verify_comment || '—' }}</div>

        <div style="margin: 16px 0; display: flex; gap: 8px; flex-wrap: wrap">
          <el-button v-if="current.status === 'reported'" type="primary" size="small" @click="act('accept')">受理</el-button>
          <el-button v-if="current.status === 'reported'" type="warning" size="small" @click="act('reject')">驳回</el-button>
          <el-button v-if="current.status === 'accepted'" type="primary" size="small" @click="act('rectify')">提交整改</el-button>
          <el-button v-if="current.status === 'rectified'" type="success" size="small" @click="act('verify')">验收通过</el-button>
          <el-button v-if="current.status === 'rectified'" type="warning" size="small" @click="act('verify_reject')">验收不合格</el-button>
          <el-button v-if="current.status === 'verified'" size="small" @click="act('close')">闭环</el-button>
        </div>

        <el-divider content-position="left">流转记录</el-divider>
        <el-timeline>
          <el-timeline-item v-for="f in flow" :key="f.id" :timestamp="fmtTime(f.created_at)" placement="top">
            <strong>{{ f.action }}</strong>
            <span class="muted"> · {{ f.actor || '—' }}</span>
            <div v-if="f.comment" class="muted">{{ f.comment }}</div>
          </el-timeline-item>
        </el-timeline>

        <el-divider content-position="left">附件</el-divider>
        <div v-if="!attachments.length" class="muted">无附件</div>
        <div v-for="a in attachments" :key="a.id" class="att-row" @click="downloadAttachment(a)">
          <span>{{ a.file_name.split(/[\\/]/).pop() }}</span>
          <span class="muted num">{{ a.file_size }} B · 下载</span>
        </div>
      </template>
    </el-drawer>
  </div>
</template>
