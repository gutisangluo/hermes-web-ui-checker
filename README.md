# Hermes Web UI Checker

Windows exe 一键检测 + 自动修复工具。双击运行，自动检测 Hermes Web UI 的 12 项前置条件并尝试修复。

## 功能

双击 `check-hermes-web-ui.exe`，自动执行 12 项检查：

| # | 检查项 | 说明 |
|---|--------|------|
| 1 | Node.js 版本 | 检查 >= 23.0.0 |
| 2 | Hermes CLI | 检查是否安装 |
| 3 | API 服务器配置 | 检查 port:8642 / host:127.0.0.1 |
| 4 | 环境变量 | 检查 GATEWAY_ALLOW_ALL_USERS=true |
| 5 | active_profile | 检查全局和 profile 目录 |
| 6 | SQLite 残留 | 清理 .db-shm/.db-wal 文件 |
| 7 | Login Lock | 清理 .login-lock.json |
| 8 | YAML 格式 | 检查全局 config.yaml |
| 9 | 多余 Profile | 删除 default profile |
| 10 | 网关运行 | 检查/启动 Hermes 网关 |
| 11 | Web UI 运行 | 检查/启动 hermes-web-ui |
| 12 | 访问信息 | 显示浏览器 URL |

## 使用方法

1. 从 Release 下载 `check-hermes-web-ui.exe`
2. 放到任意目录，**双击运行**
3. 工具自动检测并修复，最后显示访问地址

### 参数

- `-nofix` / `--nofix` / `/nofix`：仅检测，不自动修复

## 原理

- 使用 Go 交叉编译，零依赖，单文件 exe
- 通过 `wsl.exe -d Ubuntu-24.04` 指定 WSL 分发版（不依赖默认设置）
- 后台守护模式启动网关和 Web UI（nohup）
- 检测 GatewayManager 误杀网关并自动恢复

## 文件说明

- `main.go` — Go 源码，Windows 平台编译
- `go.mod` — Go 模块文件
- `check-hermes-web-ui.ps1` — PowerShell 版本（功能等价）
- `check-hermes-web-ui.bat` — PowerShell 启动器

## 构建

```bash
cd /tmp/check-hermes-web-ui
GOOS=windows GOARCH=amd64 go build -o check-hermes-web-ui.exe
```

需要 Go 1.21+，交叉编译不需要额外工具链。

## 环境要求

- Windows 10/11 + WSL2
- Hermes Agent (https://hermes-agent.nousresearch.com)
- hermes-web-ui (community edition)
- WSL 分发版: Ubuntu-24.04
