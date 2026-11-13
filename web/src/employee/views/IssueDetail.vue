<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { showToast } from 'vant'
import { api, download, getUser } from '../../shared/http'
import { fmtTime, issueStatus, severityText } from '../../shared/format'

const route = useRoute()
const issue = ref(null)
const flow = ref([])
const attachments = ref([])
const me = getUser() || {}
const showAction = ref(false)
const actionKind = ref('')
const comment = ref('')
const rectResult = ref('')
const busy = ref(false)

const isLeader = computed(() => me.role === 'admin' || me.role === 'dept_leader')

const actions = computed(() => {
  if (!issue.value) return []
  switch (issue.value.status) {
    case 'reported':
      return isLeader.value ? ['accept', 'reject'] : []
    case 'accepted':
      return ['rectify']
    case 'rectified':
      return isLeader.value ? ['verify', 'verify_reject'] : []
    case 'verified':
      return me.role === 'admin' ? ['close'] : []
    default:
      return []
  }
})

const actionText = {
  accept: '受理',
  reject: '驳回',
  rectify: '提交整改',
  verify: '验收通过',
  verify_reject: '验收不合格',
  close: '闭环',
}

onMounted(load)

async function load() {
  try {
    const [i, f, a] = await Promise.all([
      api.get(`/api/v1/issues/${route.params.id}`),
      api.get(`/api/v1/issues/${route.params.id}/flow`),
      api.get(`/api/v1/issues/${route.params.id}/attachments`),
    ])
    issue.value = i
    flow.value = f || []
    attachments.value = a || []
  } catch (e) {
    showToast(e.message)
  }
}

function openAction(kind) {
  actionKind.value = kind
  comment.value = ''
  rectResult.value = ''
  showAction.value = true
}

async function doAction() {
  const id = route.params.id
  busy.value = true
  try {
    if (actionKind.value === 'accept') {
      await api.post(`/api/v1/issues/${id}/accept`, { comment: comment.value })
    } else if (actionKind.value === 'reject') {
      await api.post(`/api/v1/issues/${id}/reject`, { comment: comment.value || '不符合上报要求' })
    } else if (actionKind.value === 'rectify') {
      await api.post(`/api/v1/issues/${id}/rectify`, { result: rectResult.value, comment: comment.value })
    } else if (actionKind.value === 'verify') {
      await api.post(`/api/v1/issues/${id}/verify`, { pass: true, comment: comment.value || '现场验收合格' })
    } else if (actionKind.value === 'verify_reject') {
      await api.post(`/api/v1/issues/${id}/verify`, { pass: false, comment: comment.value || '整改不彻底，退回' })
    } else if (actionKind.value === 'close') {
      await api.post(`/api/v1/issues/${id}/close`, { comment: comment.value })
    }
    showAction.value = false
    showToast('操作成功')
    await load()
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}

function baseName(path) {
  return String(path || '').split(/[\\/]/).pop()
}

// 附件走带令牌的 blob 下载，避免把 token 拼进 URL。
async function downloadAttachment(a) {
  try {
    await download(`/api/v1/issues/${route.params.id}/attachments/${a.id}`, baseName(a.file_name))
  } catch (e) {
    showToast(`下载失败：${e.message}`)
  }
}

async function upload(event) {
  const file = event.target.files?.[0]
  if (!file) return
  try {
    await api.upload(`/api/v1/issues/${route.params.id}/attachments`, file)
    showToast('附件已上传')
    const a = await api.get(`/api/v1/issues/${route.params.id}/attachments`)
    attachments.value = a || []
  } catch (e) {
    showToast(e.message)
  }
  event.target.value = ''
}
</script>

<template>
  <div class="app-shell">
    <van-nav-bar title="问题详情" left-arrow @click-left="$router.back()" />
    <div class="page pad">
      <template v-if="issue">
        <div class="card">
          <div class="row-between">
            <strong style="font-size: 16px">{{ issue.title }}</strong>
            <van-tag :type="issueStatus[issue.status]?.type || 'default'">{{ issueStatus[issue.status]?.text }}</van-tag>
          </div>
          <div class="muted" style="margin-top: 8px">
            {{ issue.code }} · {{ issue.cate_name || '未分类' }} · {{ severityText[issue.severity] }}
          </div>
          <div style="margin-top: 12px; line-height: 1.8; white-space: pre-wrap">{{ issue.description }}</div>
          <div class="muted" style="margin-top: 12px">
            上报：{{ issue.reporter || '—' }} · {{ fmtTime(issue.created_at) }}
          </div>
          <div class="muted">责任区队：{{ issue.dept_name || '—' }}<span v-if="issue.deadline"> · 期限 {{ fmtTime(issue.deadline) }}</span></div>
          <div v-if="issue.result" class="muted" style="margin-top: 8px">整改结果：{{ issue.result }}</div>
          <div v-if="issue.verify_comment" class="muted" style="margin-top: 4px">验收意见：{{ issue.verify_comment }}</div>
        </div>

        <div v-if="actions.length" class="card">
          <div class="row" style="flex-wrap: wrap; gap: 8px">
            <van-button
              v-for="a in actions"
              :key="a"
              size="small"
              :type="a === 'reject' || a === 'verify_reject' ? 'warning' : 'primary'"
              @click="openAction(a)"
            >
              {{ actionText[a] }}
            </van-button>
          </div>
        </div>

        <div class="card">
          <div class="row-between">
            <strong>流转记录</strong>
            <label style="color: var(--seal); font-size: 13px">
              上传附件
              <input type="file" style="display: none" @change="upload" />
            </label>
          </div>
          <div v-for="f in flow" :key="f.id" style="margin-top: 12px; padding-left: 12px; border-left: 1px solid var(--rule)">
            <div class="row-between">
              <strong style="font-size: 13px">{{ f.action }}</strong>
              <span class="muted">{{ fmtTime(f.created_at) }}</span>
            </div>
            <div class="muted">{{ f.actor || '—' }}<span v-if="f.comment">：{{ f.comment }}</span></div>
          </div>
          <div v-for="a in attachments" :key="a.id" class="att-row" @click="downloadAttachment(a)">
            <span>附件：{{ baseName(a.file_name) }}</span>
            <span class="muted num">{{ a.file_size }} B · 下载</span>
          </div>
        </div>
      </template>
    </div>

    <van-popup v-model:show="showAction" position="bottom" round>
      <div class="pad">
        <strong>{{ actionText[actionKind] }}</strong>
        <van-field
          v-if="actionKind === 'rectify'"
          v-model="rectResult"
          label="整改结果"
          type="textarea"
          rows="3"
          placeholder="整改措施与完成情况"
          style="margin-top: 10px"
        />
        <van-field
          v-model="comment"
          label="说明"
          type="textarea"
          rows="2"
          :placeholder="actionKind === 'reject' ? '驳回原因（必填）' : '备注'"
          style="margin-top: 10px"
        />
        <div style="margin-top: 14px">
          <van-button block type="primary" :loading="busy" @click="doAction">提交</van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>
