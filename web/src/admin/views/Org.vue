<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'
import { roleText } from '../../shared/format'

const tab = ref('users')

const users = ref([])
const depts = ref([])
const jobs = ref([])
const total = ref(0)
const page = reactive({ page: 1, page_size: 20 })
const userDialog = ref(false)
const userForm = reactive({ id: 0, username: '', name: '', password: '', dept_id: null, post: '', phone: '', role: 'employee', status: 'active' })

const deptDialog = ref(false)
const deptForm = reactive({ id: 0, name: '', code: '', sort: 0 })

const posts = ref([])
const postDialog = ref(false)
const postForm = reactive({ id: 0, name: '', sort: 0, status: 'active' })

async function loadPosts() {
  try {
    const data = await api.get('/api/v1/posts', { all: 1 })
    posts.value = data.list || []
  } catch (e) {
    posts.value = []
  }
}

function openPost(row) {
  if (row) {
    Object.assign(postForm, { id: row.id, name: row.name, sort: row.sort, status: row.status })
  } else {
    Object.assign(postForm, { id: 0, name: '', sort: 0, status: 'active' })
  }
  postDialog.value = true
}

async function savePost() {
  if (!postForm.name.trim()) return ElMessage.warning('岗位名称必填')
  try {
    if (postForm.id) await api.put(`/api/v1/posts/${postForm.id}`, postForm)
    else await api.post('/api/v1/posts', postForm)
    ElMessage.success('已保存')
    postDialog.value = false
    await loadPosts()
    await loadJobs()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function removePost(row) {
  try {
    await ElMessageBox.confirm(`删除岗位「${row.name}」？`, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.del(`/api/v1/posts/${row.id}`)
    ElMessage.success('已删除')
    await loadPosts()
    await loadJobs()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

const roles = ref([])
const role = ref('dept_leader')
const resources = [
  'doc', 'qa', 'learn', 'exam', 'broadcast', 'content', 'training', 'issue', 'user', 'dept', 'audit', 'task',
  'dashboard', 'eval',
]
const resourceText = {
  doc: '规程文档', qa: '智能问答', learn: '学习与小测', exam: '考试', broadcast: '宣贯', content: '安全文化内容',
  training: '新员工带培', issue: '问题闭环',
  user: '用户', dept: '部门', audit: '审计日志', task: '后台任务', dashboard: '看板', eval: '评测',
}
const roleMatrix = ref({})
const readOnlyRes = new Set(['audit', 'dashboard', 'qa'])
const adminOnlyWrite = new Set(['user', 'dept', 'task', 'eval', 'audit'])
const scopeMap = {
  audit: ['all'],
  dashboard: ['all', 'dept'],
  user: ['all', 'dept'],
  dept: ['all', 'dept'],
  task: ['all'],
  eval: ['all'],
  doc: ['all'],
  content: ['all'],
}

const roleRows = computed(() =>
  resources.map((r) => {
    const entry = roleMatrix.value[r] || { read: false, write: false, scope: 'self' }
    const writeAllowed = !readOnlyRes.has(r) && (!adminOnlyWrite.has(r) || role.value === 'admin')
    const scopes = scopeMap[r] || ['all', 'dept', 'self']
    const scope = scopes.includes(entry.scope) ? entry.scope : scopes[0]
    return { resource: r, ...entry, scope, writeAllowed, scopes }
  }),
)

async function loadUsers() {
  try {
    const data = await api.get('/api/v1/users', { ...page })
    users.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadDepts() {
  try {
    depts.value = await api.get('/api/v1/depts')
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadJobs() {
  try {
    const data = await api.get('/api/v1/posts')
    jobs.value = (data.list || []).filter((p) => p.status === 'active').map((p) => p.name)
  } catch (e) {
    jobs.value = []
  }
}

function genPassword() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'
  let s = ''
  for (let i = 0; i < 9; i++) s += chars[Math.floor(Math.random() * chars.length)]
  userForm.password = s + '@1'
  ElMessage.success('已生成初始口令，请复制给员工')
}

function onUserRole() {
  if (userForm.role === 'safety') {
    const d = depts.value.find((x) => x.name === '安全科')
    if (d) userForm.dept_id = d.id
  }
}

async function loadRoles() {
  try {
    const data = await api.get('/api/v1/roles')
    roles.value = data.roles || []
    const map = {}
    ;(data.matrix?.[role.value] || []).forEach((p) => {
      if (!map[p.resource]) map[p.resource] = { read: false, write: false, scope: p.scope }
      map[p.resource][p.action] = true
      map[p.resource].scope = p.scope
    })
    roleMatrix.value = map
  } catch (e) {
    ElMessage.error(e.message)
  }
}

const importVisible = ref(false)
const importBusy = ref(false)
const importFile = ref(null)
const importResult = ref(null)

function onImportFile(f) {
  importFile.value = f.raw
}

function downloadUserTemplate() {
  const csv = '\ufeff工号,姓名,部门,岗位,初始口令,手机号,角色\n' +
    'w1001,张三,综采一队,采煤机司机,Coal@123456,13800000000,员工\n'
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = '员工导入模板.csv'
  a.click()
  URL.revokeObjectURL(a.href)
}

async function doImport(preview) {
  if (!importFile.value) return ElMessage.warning('请选择 CSV 文件')
  importBusy.value = true
  try {
    const url = preview ? '/api/v1/users/import?preview=1' : '/api/v1/users/import'
    const res = await api.upload(url, importFile.value)
    importResult.value = res
    if (preview) {
      ElMessage.success('校验完成：可导入 ' + (res.valid || 0) + ' 条，有问题 ' + (res.invalid || 0) + ' 条')
    } else {
      ElMessage.success('导入完成：成功 ' + (res.created || 0) + ' 条')
      loadUsers()
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    importBusy.value = false
  }
}
onMounted(async () => {
  await loadUsers()
  await loadDepts()
  await loadPosts()
  await loadJobs()
  await loadRoles()
})

async function switchRole() {
  await loadRoles()
}

function openUser(row) {
  if (row) {
    Object.assign(userForm, { ...row, password: '' })
  } else {
    Object.assign(userForm, { id: 0, username: '', name: '', password: '', dept_id: null, post: '', phone: '', role: 'employee', status: 'active' })
  }
  if (userForm.role === 'safety' && !userForm.dept_id) onUserRole()
  userDialog.value = true
}

async function saveUser() {
  try {
    if (!userForm.name) {
      ElMessage.warning('请填写姓名')
      return
    }
    if (userForm.role !== 'admin' && !userForm.dept_id) {
      ElMessage.warning('员工/区队长/安全科必须选择部门')
      return
    }
    if (userForm.phone && !/^1\d{10}$/.test(userForm.phone)) {
      ElMessage.warning('手机号格式不正确')
      return
    }
    if (userForm.id) {
      await api.put(`/api/v1/users/${userForm.id}`, {
        name: userForm.name,
        dept_id: userForm.dept_id,
        post: userForm.post,
        phone: userForm.phone,
        role: userForm.role,
        status: userForm.status,
      })
    } else {
      if (!/^[A-Za-z0-9_]{4,12}$/.test(userForm.username || '')) {
        ElMessage.warning('工号需 4-12 位字母/数字/下划线')
        return
      }
      if (!userForm.password || userForm.password.length < 8 || !/[A-Za-z]/.test(userForm.password) || !/\d/.test(userForm.password)) {
        ElMessage.warning('初始口令至少 8 位，且包含字母和数字')
        return
      }
      await api.post('/api/v1/users', userForm)
    }
    ElMessage.success('已保存')
    userDialog.value = false
    await loadUsers()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function resetPassword(row) {
  try {
    const { value } = await ElMessageBox.prompt(`为 ${row.name} 设置新口令（至少 8 位）`, '重置口令', {
      inputPlaceholder: '新口令',
      inputValidator: (v) => (v && v.length >= 8) || '至少 8 位',
    })
    await api.post(`/api/v1/users/${row.id}/password`, { password: value })
    ElMessage.success('已重置并解锁')
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}

async function unlock(row) {
  try {
    await api.post(`/api/v1/users/${row.id}/unlock`)
    ElMessage.success('已解锁')
    await loadUsers()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openDept(row) {
  if (row) Object.assign(deptForm, { id: row.id, name: row.name, code: row.code, sort: row.sort })
  else Object.assign(deptForm, { id: 0, name: '', code: '', sort: 0 })
  deptDialog.value = true
}

async function saveDept() {
  if (!deptForm.name) {
    ElMessage.warning('部门名称必填')
    return
  }
  try {
    if (deptForm.id) await api.put(`/api/v1/depts/${deptForm.id}`, deptForm)
    else await api.post('/api/v1/depts', deptForm)
    ElMessage.success('已保存')
    deptDialog.value = false
    await loadDepts()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function saveRole() {
  const policies = []
  roleRows.value.forEach((r) => {
    if (r.read) policies.push({ role: role.value, resource: r.resource, action: 'read', scope: r.scope })
    if (r.write && r.writeAllowed) policies.push({ role: role.value, resource: r.resource, action: 'write', scope: r.scope })
  })
  try {
    await api.put(`/api/v1/roles/${role.value}`, { policies })
    ElMessage.success('权限已更新并即时生效')
    await loadRoles()
  } catch (e) {
    ElMessage.error(e.message)
  }
}
</script>

<template>
  <div class="panel">
    <el-tabs v-model="tab">
      <el-tab-pane label="用户" name="users">
        <div style="display: flex; margin-bottom: 12px">
          <el-button type="primary" @click="openUser(null)">新建账号</el-button>
          <el-button @click="importVisible = true">批量导入</el-button>
          <div style="flex: 1"></div>
          <span class="muted">共 {{ total }} 人</span>
        </div>
        <el-table :data="users" size="small">
          <el-table-column prop="id" label="ID" width="64" />
          <el-table-column prop="username" label="账号" width="120" />
          <el-table-column prop="name" label="姓名" width="100" />
          <el-table-column prop="dept_name" label="部门" min-width="120" />
          <el-table-column prop="post" label="岗位" min-width="150" />
          <el-table-column label="角色" width="110">
            <template #default="{ row }">{{ roleText[row.role] }}</template>
          </el-table-column>
          <el-table-column label="状态" width="86">
            <template #default="{ row }">
              <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
                {{ row.status === 'active' ? '在用' : '停用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200">
            <template #default="{ row }">
              <el-button link type="primary" @click="openUser(row)">编辑</el-button>
              <el-button link type="warning" @click="resetPassword(row)">重置口令</el-button>
              <el-button link @click="unlock(row)">解锁</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div style="margin-top: 12px; display: flex; justify-content: flex-end">
          <el-pagination
            v-model:current-page="page.page"
            v-model:page-size="page.page_size"
            :total="total"
            layout="total, prev, pager, next"
            @current-change="loadUsers"
          />
        </div>
      </el-tab-pane>

      <el-tab-pane label="部门" name="depts">
        <div style="display: flex; margin-bottom: 12px">
          <el-button type="primary" @click="openDept(null)">新建部门</el-button>
        </div>
        <el-table :data="depts" size="small">
          <el-table-column prop="id" label="ID" width="64" />
          <el-table-column prop="name" label="名称" min-width="160" />
          <el-table-column prop="code" label="编码" width="140" />
          <el-table-column prop="sort" label="排序" width="90" />
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button link type="primary" @click="openDept(row)">编辑</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="岗位" name="posts">
        <div style="display: flex; margin-bottom: 12px">
          <el-button type="primary" @click="openPost(null)">新建岗位</el-button>
          <div style="flex: 1"></div>
          <span class="muted">共 {{ posts.length }} 个</span>
        </div>
        <el-table :data="posts" size="small">
          <el-table-column prop="id" label="ID" width="64" />
          <el-table-column prop="name" label="岗位名称" min-width="180" />
          <el-table-column prop="sort" label="排序" width="90" />
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
                {{ row.status === 'active' ? '启用' : '停用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="140">
            <template #default="{ row }">
              <el-button link type="primary" @click="openPost(row)">编辑</el-button>
              <el-button link type="danger" @click="removePost(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="角色权限" name="roles">
        <div style="display: flex; gap: 10px; align-items: center; margin-bottom: 12px">
          <el-select v-model="role" style="width: 180px" @change="switchRole">
            <el-option v-for="r in roles" :key="r" :label="roleText[r] || r" :value="r" />
          </el-select>
          <el-button type="primary" @click="saveRole">保存权限</el-button>
          <span class="muted">数据范围：all=全部，dept=本部门，self=本人</span>
        </div>
        <el-table :data="roleRows" size="small">
          <el-table-column label="资源" width="180">
            <template #default="{ row }">{{ resourceText[row.resource] || row.resource }}</template>
          </el-table-column>
          <el-table-column label="查看" width="90">
            <template #default="{ row }"><el-switch v-model="row.read" /></template>
          </el-table-column>
          <el-table-column label="写入" width="90">
            <template #default="{ row }">
              <el-switch v-model="row.write" :disabled="!row.writeAllowed" />
            </template>
          </el-table-column>
          <el-table-column label="数据范围" width="180">
            <template #default="{ row }">
              <el-select v-model="row.scope" style="width: 140px">
                <el-option v-for="s in row.scopes" :key="s" :label="{ all: '全部', dept: '本部门', self: '本人' }[s]" :value="s" />
              </el-select>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="userDialog" :title="userForm.id ? '编辑账号' : '新建账号'" width="520px">
      <el-form label-width="88px">
        <el-form-item label="工号">
          <el-input v-model="userForm.username" :disabled="Boolean(userForm.id)" maxlength="12" placeholder="4-12 位字母/数字/下划线" />
        </el-form-item>
        <el-form-item v-if="!userForm.id" label="初始口令">
          <el-input v-model="userForm.password" placeholder="至少 8 位，含字母和数字" show-password>
            <template #append><el-button @click="genPassword">生成</el-button></template>
          </el-input>
        </el-form-item>
        <el-form-item label="姓名"><el-input v-model="userForm.name" /></el-form-item>
        <el-form-item label="部门" :required="userForm.role !== 'admin'">
          <el-select v-model="userForm.dept_id" placeholder="请选择部门" style="width: 100%">
            <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="岗位">
          <el-select v-model="userForm.post" filterable allow-create placeholder="选择或输入岗位" style="width: 100%">
            <el-option v-for="j in jobs" :key="j" :label="j" :value="j" />
          </el-select>
        </el-form-item>
        <el-form-item label="手机"><el-input v-model="userForm.phone" maxlength="11" placeholder="11 位手机号" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="userForm.role" style="width: 100%" @change="onUserRole">
            <el-option label="平台管理员" value="admin" />
            <el-option label="区队长" value="dept_leader" />
            <el-option label="安全科" value="safety" />
            <el-option label="培训员" value="trainer" />
            <el-option label="员工" value="employee" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="userForm.status">
            <el-radio value="active">在用</el-radio>
            <el-radio value="disabled">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <div class="muted" style="margin-left: 88px">新账号首次登录需要修改初始口令。</div>
      </el-form>
      <template #footer>
        <el-button @click="userDialog = false">取消</el-button>
        <el-button type="primary" @click="saveUser">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="deptDialog" :title="deptForm.id ? '编辑部门' : '新建部门'" width="420px">
      <el-form label-width="72px">
        <el-form-item label="名称"><el-input v-model="deptForm.name" /></el-form-item>
        <el-form-item label="编码"><el-input v-model="deptForm.code" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="deptForm.sort" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="deptDialog = false">取消</el-button>
        <el-button type="primary" @click="saveDept">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="postDialog" :title="postForm.id ? '编辑岗位' : '新建岗位'" width="420px">
      <el-form label-width="80px">
        <el-form-item label="名称"><el-input v-model="postForm.name" placeholder="如：采煤机司机" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="postForm.sort" :min="0" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="postForm.status">
            <el-radio value="active">启用</el-radio>
            <el-radio value="disabled">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <div class="muted" style="margin-left: 80px; line-height: 1.6">
          改名会同步更新账号、学习条目、风险点里的岗位名称；停用后新账号不能再选，已用账号不受影响。
        </div>
      </el-form>
      <template #footer>
        <el-button @click="postDialog = false">取消</el-button>
        <el-button type="primary" @click="savePost">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importVisible" title="批量导入员工" width="640px">
      <div class="muted" style="line-height: 1.8">
        表头：工号、姓名、部门、岗位、初始口令、手机号、角色。<br />
        部门可填名称或编码；岗位需与后台岗位配置一致；角色可填 员工/区队长/安全科/平台管理员，留空为员工；口令留空默认 Coal@123456，首次登录必须改密。
      </div>
      <div style="margin: 10px 0">
        <el-button size="small" @click="downloadUserTemplate">下载模板</el-button>
      </div>
      <el-upload :auto-upload="false" :limit="1" accept=".csv" :on-change="onImportFile">
        <el-button>选择 CSV 文件</el-button>
      </el-upload>
      <div v-if="importResult" style="margin-top: 12px">
        <el-alert
          :title="importResult.preview
            ? `校验完成：可导入 ${importResult.valid} 条，有问题 ${importResult.invalid} 条`
            : `导入完成：成功 ${importResult.created} 条，跳过 ${importResult.skipped} 条`"
          type="success"
          :closable="false"
        />
        <el-table v-if="importResult.errors && importResult.errors.length" :data="importResult.errors" size="small" max-height="220" style="margin-top: 8px">
          <el-table-column prop="row" label="行" width="60" />
          <el-table-column prop="username" label="工号" width="120" />
          <el-table-column prop="reason" label="原因" />
        </el-table>
      </div>
      <template #footer>
        <el-button @click="importVisible = false">关闭</el-button>
        <el-button :loading="importBusy" @click="doImport(true)">预览校验</el-button>
        <el-button type="primary" :loading="importBusy" @click="doImport(false)">开始导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>
