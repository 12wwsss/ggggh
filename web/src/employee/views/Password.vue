<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api, getUser, saveSession, getToken } from '../../shared/http'

const router = useRouter()
const pwd = ref('')
const confirm = ref('')
const busy = ref(false)

async function submit() {
  if (pwd.value.length < 8) return showToast('新口令至少 8 位')
  if (pwd.value !== confirm.value) return showToast('两次输入不一致')
  busy.value = true
  try {
    await api.post('/api/password/first', { password: pwd.value })
    const u = { ...(getUser() || {}), must_change_password: false }
    saveSession(getToken(), u)
    showToast('口令已修改')
    router.replace('/')
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <div class="login-card">
      <h1>设置新口令</h1>
      <p>首次登录需要修改初始口令，至少 8 位</p>
      <van-cell-group inset>
        <van-field v-model="pwd" type="password" label="新口令" placeholder="至少 8 位" />
        <van-field v-model="confirm" type="password" label="再输一次" placeholder="再次输入新口令" />
      </van-cell-group>
      <div style="margin-top: 16px">
        <van-button round block type="primary" :loading="busy" @click="submit">保存并进入</van-button>
      </div>
    </div>
  </div>
</template>
