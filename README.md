# rsi - Mihomo Router Manager CLI

[![Release](https://github.com/WASIDJ/rsi/actions/workflows/release.yml/badge.svg)](https://github.com/WASIDJ/rsi/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A lightweight, blazing-fast Go CLI tool designed for **Mihomo (Clash Meta) on routers** (ASUS RT-AX86U / Merlin / OpenWrt / Linux ARM64).

It supports:
- **Zero-Downtime Hot Reload** via Mihomo REST API (`PUT /configs`) in < 0.2s without restarting the service or dropping connections.
- **Airport Subscription Management**: Fetch, parse, adapt to router rules, and apply with a single command.
- **Hysteria 2 & Custom Node Injection**: Add nodes via `hysteria2://` URI or YAML, automatically creating dedicated selector groups and prepending them to all major routing groups.
- **Dual-Mode Operation**: Run natively on the router, or run locally on macOS/Linux to control the router remotely via SSH.

---

## Installation

### Method 1: Homebrew (macOS)

```sh
# Direct installation via Homebrew formula
brew install WASIDJ/rsi/rsi
```

### Method 2: Install to Router (ARM64)

From your Mac terminal:
```sh
# Deploy directly to the router
rsi router deploy
```
Or build manually:
```sh
make deploy
```

---

## Quick Start & Usage

### 1. 机场订阅管理 (Airport Subscriptions)

```sh
# 换新机场：一键拉取、融合自建节点、语法自检、秒级热重载
rsi sub set "https://your-airport.com/api/v1/client/subscribe?token=xxx"

# 日常更新：按已保存的订阅链接拉取最新节点
rsi sub update

# 查看当前已保存的机场订阅链接
rsi sub show
```

### 2. 自建 Hysteria 2 (hy2) 节点接入

支持直接粘贴 `hysteria2://` 链接：
```sh
# 1. 直接添加 hy2 节点
rsi node add "hysteria2://password@tokyo-hy2.example.com:443/?sni=tokyo-hy2.example.com&insecure=0#⚡ 自建-东京-HY2"

# 2. 查看当前已配置的自建节点
rsi node list

# 3. 删除指定自建节点并自动热重载
rsi node rm "⚡ 自建-东京-HY2"

# 4. 从 YAML 文件批量导入
rsi node add -f custom_nodes.yaml
```

> **自动注入机制**：
> 任何自建节点加入后，会在面板最顶层自动生成 `⚡ 自建节点` 策略组，并自动插到 `♻️ 手动切换`、`🔎 Google`、`🧲 OpenAI`、`🎬 YouTube` 等核心策略组的候选最顶端。更新机场订阅时，自建节点会被永久保留。

### 3. 运维与诊断
 
```sh
# 查看核心运行状态、PID、内存占用 (VmRSS)、自建节点状态
rsi status

# 查看国内 IP 直连规则集状态或一键热更新
rsi chnroute status
rsi chnroute update

# 查看防火墙分流计数与博通 Flow Cache 硬件加速状态
rsi firewall status
rsi firewall apply

# 重新验证并热重载当前配置
rsi reload

# 查看或追踪实时日志
rsi log -f

# 启停与重启
rsi restart
rsi stop
rsi start
```

---

## macOS Remote Control

When running on macOS, `rsi` automatically forwards commands to your router via SSH (default target: `RSI@192.168.50.1`).

You can customize the target using the `RSI_TARGET` environment variable:
```sh
export RSI_TARGET="admin@192.168.1.1"
rsi status
```

---

## License

MIT License
