<p align="center">
  <a href="https://s.flyfish.dev">
    <img src="web/static/brand.svg" width="76" height="76" alt="AI Shortlink logo">
  </a>
</p>

<h1 align="center">AI Shortlink</h1>

<p align="center">
  <strong>让每一个公开入口，在传播之后仍然可控。</strong>
  <br>
  Keep every shared entry point controllable after it leaves your hands.
</p>

<p align="center">
  <a href="README.md">简体中文</a> · <a href="README.en.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/flyfish-dev/shortlink/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/flyfish-dev/shortlink/ci.yml?branch=main&style=flat-square&label=CI"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0--only-5A31F4?style=flat-square"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-ready-2496ED?style=flat-square&logo=docker&logoColor=white">
  <a href="https://github.com/flyfish-dev/shortlink/issues"><img alt="Issues" src="https://img.shields.io/github/issues/flyfish-dev/shortlink?style=flat-square"></a>
  <a href="https://s.flyfish.dev"><img alt="Production" src="https://img.shields.io/badge/production-s.flyfish.dev-111827?style=flat-square"></a>
</p>

<p align="center">
  <a href="https://s.flyfish.dev">在线体验</a>
  · <a href="#为什么是-ai-shortlink">品牌理念</a>
  · <a href="#核心能力">核心能力</a>
  · <a href="#快速开始">快速开始</a>
  · <a href="ROADMAP.md">开发计划</a>
  · <a href="CONTRIBUTING.md">参与贡献</a>
</p>

## 产品预览

![AI Shortlink 功能录屏](docs/assets/ai-shortlink-demo.gif)

录屏来自真实实例，覆盖总览、短链管理、折叠操作、活码二维码池、品牌二维码定制、多格式下载和系统设置。可直接访问 [s.flyfish.dev](https://s.flyfish.dev) 体验当前生产版本。

## 为什么是 AI Shortlink

链接和二维码一旦被印刷、发布或转发，就很难回收；但它们背后的目标、权限和运营策略仍会变化。AI Shortlink 将短链和活码视为长期运营的数字入口，而不是一次性的转换工具。

产品坚持四个原则：

- **稳定入口，持续演进**：入口保持不变，目标、二维码池和分发策略可以安全调整。
- **控制先于自动化**：所有自动化都应可预览、可确认、可撤销，并留下审计记录。
- **边界清晰**：普通用户只管理自己创建的资源，管理员负责全局设置与审核。
- **轻量但完整**：单体 Go 服务、内置管理端和 SQLite 可快速部署，同时保留 MySQL/MariaDB 的生产扩展路径。

### “AI” 具体指什么

[Issue #1](https://github.com/flyfish-dev/shortlink/issues/1) 提出了一个重要问题：AI Shortlink 中的 AI 是否仅仅表示“项目由 AI 编写”？答案是：**不止于开发方式，也不等于当前版本已经把所有功能 AI 化。**

AI 在这里有两层含义：

1. **工程方式**：项目采用人类主导、AI 协作的开发方式，加快设计、实现、测试和文档迭代；最终判断与责任始终由维护者承担。
2. **产品方向**：逐步引入可选的大模型能力，辅助完成自然语言配置、审核信息整理、风险提示和运营洞察。

当前版本聚焦可靠的链接基础设施，**默认不调用任何大模型 API，也不会把链接或用户数据发送给第三方模型服务**。未来 AI 能力将遵循显式开启、最小数据、人工确认、结果可解释、操作可审计的原则。完整阶段与边界见 [ROADMAP.md](ROADMAP.md)。

## 核心能力

| 领域 | 能力 |
| --- | --- |
| 短链管理 | 自定义短码、301/302/307/308、启停与有效期、访问上限、备用链接、二维码与访问统计 |
| 活码运营 | 固定入口、多二维码池、排序与权重、轮询/随机/最少展示策略、事务化保存 |
| 品牌二维码 | classic / rounded / dots、前景与背景色、中心贴图、实时预览、SVG / PNG / WEBP 下载 |
| 团队协作 | 多用户、资源所有权、管理员审核、通过/驳回邮件通知、操作审计 |
| 安全登录 | Magic Link、GitHub OAuth、可信浏览器一键登录、恢复 Key、账户状态控制 |
| 数据洞察 | 近 30 日趋势、独立 IP、设备与浏览器分布、活码展示与长按识别意图 |
| 私有部署 | Docker、Linux amd64 二进制、Render、SQLite、MySQL / MariaDB |
| 国际化 | 中英文产品名称、界面与邮件文案，支持自动匹配浏览器语言 |

## 快速开始

最轻量的方式是使用 Docker 和内置 SQLite：

```bash
git clone https://github.com/flyfish-dev/shortlink.git
cd shortlink
cp .env.example .env
docker compose up -d --build
```

访问 `http://localhost:8080/setup`，按向导配置站点与管理员。运行数据保存在 Docker volume 中。

本地编译需要 Go 1.23+、C 编译器和 SQLite3 开发库：

```bash
make test
make build
./bin/ai-shortlink
```

MySQL/MariaDB、GitHub OAuth、HTTPS 反向代理、systemd 和 Render 部署说明见 [部署指南](docs/DEPLOYMENT.md)。

## 系统边界

```text
公开访问者 ──> /s/{code} 短链跳转
          └──> /q/{code} 活码展示

普通用户 ────> 仅查看和维护自己创建的短链、活码与统计
管理员 ──────> 全部资源、用户、系统设置与审核

SQLite / MySQL <── Go 单体服务 ──> 内置 HTML / CSS / JavaScript 管理端
```

关键约束：

- 普通用户的列表、详情、统计、编辑和删除均受 `owner_account_id` 限制，不能跨账户访问资源。
- 新建或修改的短链、活码和二维码项进入待审状态；未通过审核的入口不会公开生效。
- 二维码中心贴图会保留必要的纠错与留白，相关渲染逻辑覆盖真实解码测试。
- Magic Link 是一次性且限时的；GitHub OAuth 只读取登录所需的用户资料和已验证邮箱。

更完整的模块、请求流和数据边界见 [架构说明](docs/ARCHITECTURE.md) 与 [用户及权限模型](docs/USER_MANAGEMENT.md)。

## AI 能力演进

| 阶段 | 目标 | 状态 |
| --- | --- | --- |
| 可信入口基础 | 短链、活码、品牌二维码、权限、审核、统计、登录与部署 | 已交付，持续加固 |
| AI 接入基础 | 可选模型供应商、明确的数据范围、提示词版本、调用审计与失败降级 | 规划中 |
| 配置助手 | 将自然语言意图转换为可检查的短链/活码配置草稿，由用户确认后保存 | 规划中 |
| 审核助手 | 汇总目标信息、给出风险信号与审核建议，默认不替代管理员决定 | 规划中 |
| 运营洞察 | 趋势摘要、异常提醒和可追溯的优化建议 | 探索中 |

路线图不是发布日期承诺。每项能力会先形成可讨论的 Issue，再进入实现；欢迎通过 [Feature request](https://github.com/flyfish-dev/shortlink/issues/new?template=feature_request.yml) 补充真实场景。

## 文档导航

| 文档 | 适合谁 | 内容 |
| --- | --- | --- |
| [部署指南](docs/DEPLOYMENT.md) | 使用者、运维 | Docker、二进制、数据库、OAuth、SMTP、HTTPS |
| [架构说明](docs/ARCHITECTURE.md) | 开发者 | 模块边界、请求流、数据模型、关键设计决策 |
| [用户及权限模型](docs/USER_MANAGEMENT.md) | 管理员、开发者 | 角色、资源隔离、审核与二维码样式 |
| [开发路线图](ROADMAP.md) | 使用者、贡献者 | 产品阶段、AI 原则、近期优先级与非目标 |
| [贡献指南](CONTRIBUTING.md) | 贡献者 | 环境搭建、分支、测试、PR 与文档要求 |
| [安全策略](SECURITY.md) | 安全研究者 | 支持范围、漏洞报告和响应流程 |
| [支持指南](SUPPORT.md) | 所有人 | 问题分类、提问信息与维护范围 |
| [行为准则](CODE_OF_CONDUCT.md) | 社区参与者 | 协作标准与执行原则 |

## 参与贡献

建议先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)，再选择合适入口：

- 可复现的缺陷：[Bug report](https://github.com/flyfish-dev/shortlink/issues/new?template=bug_report.yml)
- 产品建议与 AI 场景：[Feature request](https://github.com/flyfish-dev/shortlink/issues/new?template=feature_request.yml)
- 使用问题：[Question](https://github.com/flyfish-dev/shortlink/issues/new?template=question.yml)
- 安全问题：请勿公开披露，按 [SECURITY.md](SECURITY.md) 私下报告

提交前至少运行：

```bash
go test ./...
go vet ./...
go build -o bin/ai-shortlink ./cmd/server
node --check web/static/app.js
node --check web/static/platform_ext.js
```

## 开源许可

AI Shortlink 采用 [GNU Affero General Public License v3.0 only](LICENSE)（SPDX: `AGPL-3.0-only`）开源。修改、分发或通过网络提供本项目服务时，需要遵守 AGPL-3.0-only 的源码公开与版权告知要求。版权和源码声明见 [NOTICE](NOTICE)。

Copyright (C) 2026 [Flyfish Dev](https://flyfish.dev)
