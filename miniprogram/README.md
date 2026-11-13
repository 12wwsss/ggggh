# 微信小程序外壳

给“没有网络常识”的用户用：打开小程序 → 微信授权 → 首次绑工号 → 直接进 H5，之后免登录。

## 文件

```
app.js / app.json          入口与页面注册
pages/login/               微信登录 + 首次绑定工号
pages/index/               web-view 打开现有员工端 H5
project.config.json        开发者工具配置（appid 需替换）
```

部署与域名配置见 `deploy/README.md`；备案完成后先打通隧道，再改下面三处。

## 上线前要改三处

1. `app.js` 里的 `base`：改成已备案的 https 域名，例如 `https://rag.example.com`；
2. `project.config.json` 的 `appid`：填真实 AppID（现在是 `touristappid`，只能本地预览）；
3. 后端环境变量：`COALRAG_WX_APPID`、`COALRAG_WX_SECRET`（未配置时为桩模式，本地联调可用）。

## 小程序后台配置

| 配置项 | 值 |
| --- | --- |
| request 合法域名 | `https://rag.example.com` |
| 业务域名（web-view） | 同上，且能访问到校验文件 |
| downloadFile 合法域名 | 同上（附件下载用） |

## 登录流程

```
wx.login → code
   └─ POST /api/v1/auth/wx-login
         ├─ 已绑定 → {token, user, ticket}
         └─ 未绑定 → {need_bind:true, bind_ticket}
               └─ 工号+口令 → POST /api/v1/auth/wx-bind → {token, user, ticket}
   └─ web-view 打开 https://域名/?ticket=<一次性票据>
          └─ H5 自动 POST /api/v1/auth/ticket 换会话，地址栏随即清理
```

票据 2 分钟有效、只能用一次；JWT 不放进 URL，避免被网关或日志记录。

## 限制

- web-view 内不能调用微信原生能力（扫码、定位），需要时得做成原生页面；
- 域名必须备案且 HTTPS，证书私钥留在矿内；
- 小程序类目报“工具/教育”更容易过审，安全生产类可能要求资质。
