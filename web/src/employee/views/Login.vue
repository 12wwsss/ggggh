<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast } from 'vant'
import { login } from '../../shared/http'

const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const loading = ref(false)

async function submit() {
  if (!username.value || !password.value) {
    showToast('请输入账号与口令')
    return
  }
  loading.value = true
  try {
    const data = await login(username.value.trim(), password.value)
    if (data?.user?.must_change_password) {
      router.replace('/password')
      return
    }
    const target = route.query.redirect || '/'
    router.replace(target)
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <div class="login-card">
      <h1>煤矿安全员助手</h1>
      <p>岗位学习 · 规程问答 · 隐患上报</p>
      <van-form @submit="submit">
        <van-cell-group inset>
          <van-field v-model="username" label="账号" placeholder="请输入账号" autocomplete="username" />
          <van-field
            v-model="password"
            type="password"
            label="口令"
            placeholder="请输入口令"
            autocomplete="current-password"
          />
        </van-cell-group>
        <div style="margin-top: 16px">
          <van-button round block type="primary" native-type="submit" :loading="loading">登录</van-button>
        </div>
      </van-form>
      <p style="margin-top: 14px">演示账号 worker01 / Worker@123456</p>
    </div>
  </div>
</template>
