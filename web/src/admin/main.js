import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import '../shared/theme.css'
import App from './App.vue'
import router from './router'
import { onUnauthorized } from '../shared/http'

onUnauthorized(() => {
  if (router.currentRoute.value.name !== 'login') router.replace('/login')
})

createApp(App).use(ElementPlus, { locale: zhCn }).use(router).mount('#app')
