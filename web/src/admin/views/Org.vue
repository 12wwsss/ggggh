<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'
import { roleText } from '../../shared/format'

const tab = ref('users')

const users = ref([])
const depts = ref([])
const total = ref(0)
const page = reactive({ page: 1, page_size: 20 })
const userDialog = ref(false)
const userForm = reactive({ id: 0, username: '', name: '', password: '', dept_id: null, post: '', phone: '', role: 'employee', status: 'active' })

const deptDialog = ref(false)
const deptForm = reactive({ id: 0, name: '', code: '', sort: 0 })

const roles = ref([])
const role = ref('dept_leader')
const resources = [
  'doc', 'qa', 'learn', 'exam', 'broadcast', 'issue', 'user', 'dept', 'audit', 'task', 'dashboard', 'eval',
]
const resourceText = {
  doc: '规程文档', qa: '智能问答', learn: '学习与小测', exam: '考试', broadcast: '宣贯', issue: '问题闭环',
  user: '用户', dept: '部门', audit: '审计日志', task: '后台任务', dashboard: '看板', eval: '评测',
}
const roleMatrix = ref({})

const roleRows = computed(() =>
  resources.map((r) => {
    const entry = roleMatrix.value[r] || { read: false, write: false, scope: 'self' }
    return { resource: r, ...entry }
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

onMounted(async () => {
  await loadUsers()
  await loadDepts()
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
  userDialog.value = true
}

async function saveUser() {
  try {
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
      if (!userForm.username || !userForm.password) {
        ElMessage.warning('账号与初始口令必填')
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
    if (r.write) policies.push({ role: role.value, resource: r.resource, action: 'write', scope: r.scope })
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
            <template #default="{ row }"><el-switch v-model="row.write" /></template>
          </el-table-column>
          <el-table-column label="数据范围" width="180">
            <template #default="{ row }">
              <el-select v-model="row.scope" style="width: 140px">
                <el-option label="全部" value="all" />
                <el-option label="本部门" value="dept" />
                <el-option label="本人" value="self" />
              </el-select>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="userDialog" :title="userForm.id ? '编辑账号' : '新建账号'" width="520px">
      <el-form label-width="88px">
        <el-form-item label="账号">
          <el-input v-model="userForm.username" :disabled="Boolean(userForm.id)" />
        </el-form-item>
        <el-form-item v-if="!userForm.id" label="初始口令">
          <el-input v-model="userForm.password" placeholder="至少 8 位" show-password />
        </el-form-item>
        <el-form-item label="姓名"><el-input v-model="userForm.name" /></el-form-item>
        <el-form-item label="部门">
          <el-select v-model="userForm.dept_id" style="width: 100%">
            <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="岗位"><el-input v-model="userForm.post" /></el-form-item>
        <el-form-item label="手机"><el-input v-model="userForm.phone" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="userForm.role" style="width: 100%">
            <el-option label="平台管理员" value="admin" />
            <el-option label="区队长" value="dept_leader" />
            <el-option label="员工" value="employee" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="userForm.status">
            <el-radio value="active">在用</el-radio>
            <el-radio value="disabled">停用</el-radio>
          </el-radio-group>
        </el-form-item>
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
  </div>
</template>
