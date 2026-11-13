<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { showDialog, showToast } from 'vant'
import { api, clearSession, getUser } from '../../shared/http'
import { roleText } from '../../shared/format'
import { useLarge } from '../../shared/large'

const router = useRouter()
const user = ref(getUser() || {})
const oldPassword = ref('')
const newPassword = ref('')
const busy = ref(false)
const large = useLarge()

async function changePassword() {
  if (!oldPassword.value || !newPassword.value) {
    showToast('请填写原口令与新口令')
    return
  }
  if (newPassword.value.length < 8) {
    showToast('新口令至少 8 位')
    return
  }
  busy.value = true
  try {
    await api.post('/api/v1/auth/password', {
      old_password: oldPassword.value,
      new_password: newPassword.value,
    })
    oldPassword.value = ''
    newPassword.value = ''
    showToast('口令已更新')
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}

async function logout() {
  try {
    await showDialog({ title: '退出登录', message: '确定要退出当前账号吗？' })
  } catch {
    return
  }
  clearSession()
  router.replace('/login')
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="我的" />
    <div class="pad">
      <div class="card">
        <div class="row">
          <div
            class="icon"
            style="width: 48px; height: 48px; border-radius: 50%; background: var(--seal-tint); color: var(--seal); font-family: var(--font-serif)"
          >
            {{ (user.name || '员')[0] }}
          </div>
          <div>
            <strong style="font-size: 16px">{{ user.name }}</strong>
            <div class="muted">{{ user.dept_name }} · {{ user.post || roleText[user.role] }}</div>
          </div>
        </div>
      </div>

      <van-cell-group inset style="margin-bottom: 12px">
        <van-cell title="账号" :value="user.username" />
        <van-cell title="角色" :value="roleText[user.role] || user.role" />
        <van-cell title="智能问答" is-link @click="router.push('/ask')" />
        <van-cell title="我的问题" is-link @click="router.push('/issues')" />
        <van-cell title="大字模式" label="字更大、按钮更大">
          <template #right-icon>
            <van-switch :model-value="large" size="26" @update:model-value="(v) => (large = v)" />
          </template>
        </van-cell>
      </van-cell-group>

      <div class="card">
        <strong>修改口令</strong>
        <van-field v-model="oldPassword" type="password" label="原口令" placeholder="请输入原口令" style="margin-top: 8px" />
        <van-field v-model="newPassword" type="password" label="新口令" placeholder="至少 8 位" />
        <div style="margin-top: 12px">
          <van-button block size="small" type="primary" :loading="busy" @click="changePassword">保存</van-button>
        </div>
      </div>

      <div style="margin: 16px 0">
        <van-button block plain type="danger" @click="logout">退出登录</van-button>
      </div>
    </div>
  </div>
</template>
