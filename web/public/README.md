# 域名校验文件放这里

微信小程序配置「业务域名」时会要求把一个校验文件（形如 `abc123def.txt`）放到域名根目录，
使 `https://你的域名/abc123def.txt` 能直接访问。

做法：

1. 把微信给的文件原样放进本目录（`web/public/`）；
2. 重新构建前端：`pnpm build`（或 `node node_modules/vite/bin/vite.js build`）；
3. 重新编译服务端（前端已 `go:embed`）：`go build -o bin/coalrag-server.exe ./cmd/server`；
4. 重启服务，访问 `https://你的域名/abc123def.txt` 应返回文件内容（`text/plain`）。

注意：

- 文件必须放在根路径，不能带子目录；
- 缺失时服务返回 404 而不是首页，便于发现配置漏了；
- 校验通过后文件可以保留，微信会周期性复查。
