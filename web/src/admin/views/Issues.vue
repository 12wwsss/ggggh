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

const actionText = {
  report: '上报',
  accept: '受理',
  reject: '驳回',
  rectify: '提交整改',
  verify: '验收通过',
  verify_reject: '验收不合格',
  close: '闭环',
  reassign: '更换负责人',
  admin_force_rectify: '管理员代整改',
  admin_force_verify: '管理员强制验收',
  admin_force_close: '管理员强制闭环',
}

// 与后端状态机保持一致：只有还能往下走的单据才叫“处理”，
// 已闭环 / 已驳回的只能看，按钮文案改成“查看”。
// 与后端状态机保持一致：区队长、安全科、管理员可以受理/验收/闭环；
// 整改只能由指派人提交，界面不给无关角色显示按钮。
const me = getUser()
const isManager = (role) => role === 'admin' || role === 'dept_leader' || role === 'safety'

function canAct(row) {
  const role = me?.role
  switch (row.status) {
    case 'reported':
      return isManager(role)
    case 'accepted':
      return role === 'admin' || (row.assignee_id && me?.id === row.assignee_id)
    case 'rectified':
      return isManager(role)
    case 'verified':
      return isManager(role)
    default:
      return false
  }
}

function canRectify() {
  if (!current.value || current.value.status !== 'accepted') return false
  return me?.role === 'admin' || me?.id === current.value.assignee_id
}

const acceptDialog = ref(false)
const acceptForm = reactive({ assignee_id: null, comment: '' })
const staff = ref([])

async function startAccept() {
  acceptForm.assignee_id = current.value?.reporter_id || null
  acceptForm.comment = ''
  acceptDialog.value = true
  try {
    const data = await api.get('/api/v1/users', { page_size: 200 })
    staff.value = data.list || []
  } catch {
    staff.value = []
  }
}

async function confirmAccept() {
  try {
    await api.post(`/api/v1/issues/${current.value.id}/accept`, {
      assignee_id: acceptForm.assignee_id,
      comment: acceptForm.comment,
    })
    ElMessage.success('已受理')
    acceptDialog.value = false
    await load()
    current.value = await api.get(`/api/v1/issues/${current.value.id}`)
    flow.value = await api.get(`/api/v1/issues/${current.value.id}/flow`)
  } catch (e) {
    ElMessage.error(e.message)
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

const reassignVisible = ref(false)
const reassignForm = reactive({ assignee_id: null, reason: '' })
const reassignStaff = ref([])

async function openReassign() {
  reassignForm.assignee_id = null
  reassignForm.reason = ''
  reassignStaff.value = []
  reassignVisible.value = true
  try {
    const params = { page_size: 200 }
    if (current.value && current.value.dept_id) params.dept_id = current.value.dept_id
    const data = await api.get('/api/v1/users', params)
    reassignStaff.value = (data.list || []).filter((u) => u.status === 'active')
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function submitReassign() {
  if (!reassignForm.assignee_id) return ElMessage.warning('请选择新的负责人')
  try {
    await api.post(`/api/v1/issues/${current.value.id}/reassign`, reassignForm)
    ElMessage.success('已更换负责人')
    reassignVisible.value = false
    await open(current.value)
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
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
      return startAccept()
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
      <div style="flex: 1"></div>
      <el-button @click="download('/api/v1/export/issues', '隐患台账.xlsx')">导出 Excel</el-button>
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
        <div class="muted">整改时限：{{ current.deadline ? fmtTime(current.deadline) : '未设置（受理时默认 24 小时）' }}</div>
        <div class="muted">整改结果：{{ current.result || '—' }}</div>
        <div class="muted">验收意见：{{ current.verify_comment || '—' }}</div>

        <div style="margin: 16px 0; display: flex; gap: 8px; flex-wrap: wrap">
          <el-button v-if="current.status === 'reported'" type="primary" size="small" @click="startAccept">受理</el-button>
          <el-button v-if="current.status === 'reported'" type="warning" size="small" @click="act('reject')">驳回</el-button>
          <el-button v-if="current.status === 'accepted' && canRectify()" type="primary" size="small" @click="act('rectify')">提交整改</el-button>
          <el-button v-if="current.status === 'rectified'" type="success" size="small" @click="act('verify')">验收通过</el-button>
          <el-button v-if="current.status === 'rectified'" type="warning" size="small" @click="act('verify_reject')">验收不合格</el-button>
          <el-button v-if="current.status === 'verified'" size="small" @click="act('close')">闭环</el-button>
          <el-button v-if="current.status === 'accepted'" size="small" plain @click="openReassign">更换负责人</el-button>
        </div>

        <el-divider content-position="left">流转记录</el-divider>
        <el-timeline>
          <el-timeline-item v-for="f in flow" :key="f.id" :timestamp="fmtTime(f.created_at)" placement="top">
            <strong>{{ actionText[f.action] || f.action }}</strong>
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

    <el-dialog v-model="acceptDialog" title="受理并指派整改人" width="460px">
      <el-form label-width="80px">
        <el-form-item label="整改人">
          <el-select v-model="acceptForm.assignee_id" filterable style="width: 100%" placeholder="选择责任人">
            <el-option v-for="u in staff" :key="u.id" :label="`${u.name}（${u.username}）`" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="受理意见">
          <el-input v-model="acceptForm.comment" type="textarea" :rows="2" placeholder="可留空" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="acceptDialog = false">取消</el-button>
        <el-button type="primary" @click="confirmAccept">确认受理</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="reassignVisible" title="更换整改负责人" width="460px">
      <el-form label-width="90px">
        <el-form-item label="新负责人">
          <el-select v-model="reassignForm.assignee_id" filterable style="width: 100%" placeholder="选择本队人员">
            <el-option v-for="u in reassignStaff" :key="u.id" :label="`${u.name}（${u.username}）`" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="更换原因">
          <el-input v-model="reassignForm.reason" type="textarea" :rows="2" placeholder="如：原负责人已调岗" />
        </el-form-item>
        <div class="muted">更换后会通知原负责人和新负责人，并记录一条流转。</div>
      </el-form>
      <template #footer>
        <el-button @click="reassignVisible = false">取消</el-button>
        <el-button type="primary" @click="submitReassign">确认更换</el-button>
      </template>
    </el-dialog>
  </div>
</template>
