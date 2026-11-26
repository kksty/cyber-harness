# 配置与初始化

配置是 **cyber-harness 的通用功能**。下文使用 `cyber-scan` 举例；通用 `agent` CLI 支持相同的配置命令，嵌入式宿主可直接使用公共配置包。

## 首次配置

```sh
cyber-scan init
cyber-scan init --non-interactive --provider openai --model your-model
cyber-scan init --project
```

`init` 默认写 `~/.cyber/cyber.yaml`。在交互终端中引导填写协议、端点、模型和 API key；密钥输入隐藏，留空可使用环境变量。非终端或 `--non-interactive` 不进入问答，只写显式参数，不读取或持久化环境变量凭据。初始化不会向模型服务发请求。

`init --project` 在当前目录创建最小 `cyber.yaml`，不复制用户模型、个人凭据或运行时配置。未填写的示例保持注释，不会阻止 `OPENAI_*`、`ANTHROPIC_*` 补齐配置。

已有文件默认保持不变。`--force` 创建独立备份后原子替换；失败保留旧文件。`-c <path> init` 创建指定文件，不能与 `--project` 同用。

可选的执行检查见[工具审批配置](#工具审批配置)。

## 文件与优先级

```text
CLI > CYBER_*／集成专用环境变量 > 当前目录配置 > 用户配置
    > 协议环境变量 > 默认值
```

用户文件为 `~/.cyber/cyber.yaml`。当前目录先查找 `cyber.yaml`，不存在时查找 `.cyber/cyber.yaml`，只加载其中一份并叠加到用户配置上，同名配置项以当前目录为准。不向父目录或可执行文件目录查找配置。通用 agent 的 `--workdir` 决定这里的工作目录。

没有当前目录配置时使用用户配置。显式 `-c` 仅加载指定文件，不继承用户配置；仍允许环境变量和 CLI 覆盖。

普通映射按字段合并，列表整体替换。`llm.providers` 按 ID 合并：项目可以只覆盖某个 profile 的模型，继承其端点与凭据。未填写 ID 时按顺序分配 `profile-1` 等 ID；多层配置应显式填写 ID。

文件中空的 LLM 协议、端点、模型、密钥字符串按未填写处理，允许低优先级来源补齐。显式 CLI 空值表示清空；`false`、`0`、空列表不会被当成缺省值。

扩展配置直接使用同名顶层块，例如 `scan`、`search`、`recon`、`ioa.client`。解析、展示和保存统一使用这一格式，未注册的配置块和未知字段报错。外部 API 的进程内接管使用 [gateway](developer/gateway.md)，字段直接写在服务名下。`gateway.<source>.base_url` 支持 passive 的全部 16 个来源及 Tavily、DuckDuckGo；`jev.base_url` 配置 TypeSafe 上游。FOFA、Hunter、Tavily 和 JEV 的连接检查使用相同配置地址。

## 模型 profiles

```yaml
llm:
  active_profile: work
  providers:
    - id: work
      provider: openai
      base_url: https://api.example.com/v1
      model: your-model
```

```sh
cyber-scan config profiles
cyber-scan agent --profile work --model another-model -p "列出当前目录"
cyber-scan config use work
cyber-scan config use work --project
```

选择 profile 后逐字段应用覆盖，`--model` 不会丢失端点、密钥、代理和限额。协议环境变量只补齐所选协议的缺失值；其他协议的凭据不会改变已选 profile。没有显式选择时使用第一项，错误 ID 和重复 ID 报错，不自动切换模型重试。

`config use` 默认保存用户级选择，`--project` 修改当前目录中已加载的配置，无当前目录配置则创建 `cyber.yaml`；`-c` 修改指定文件。用户级选择不能引用仅存在于项目的 profile。更高层覆盖保存结果时会提示。

`model` 为必填字段，保存或激活 Profile 时校验。REPL `/provider` 查看当前配置，`/model` 仅选择当前会话下一次运行的模型；其他会话和已启动子任务保留各自快照。端点、凭据和 Provider 在文件或 Web 设置中修改。Web 模型列表使用当前编辑 Profile 的已保存凭据；端点不提供 `GET /models` 时保留手动模型输入。

## 查看与检查

```sh
cyber-scan config show --sources
cyber-scan config show --sources --json
cyber-scan config validate
cyber-scan doctor
cyber-scan doctor --online
```

`show` 展示有效配置，密钥及 URL 凭据始终脱敏；`--sources` 辅助定位文件、环境变量或 CLI 覆盖。`validate` 离线检查字段、类型及 profile 引用。未知字段和当前发行版未注册的配置块均报错。

`doctor` 默认检查配置和目录条件，不访问网络、不启动扫描；`--online` 才执行模型及宿主提供的已配置连接检查。未配置可选模型会跳过连接测试。检查失败退出码为 1；成功为 0。命令支持 `--json`，提示信息写 stderr。

## Inbox 子命令与持久状态

```yaml
inbox:
  state_dir: .cyber/active
  max_active: 4         # 正在执行的根任务数；Inbox 等待释放名额，同一任务始终串行
agent:
  timeout: 0           # 共用执行设置，秒；inbox 默认 0，agent 默认 3600
```

通过 `cyber-scan inbox` 启动常驻 Router，`cyber-scan agent` 保留现有 prompt／REPL 行为；配置文件不能切换启动子命令。`inbox.state_dir` 与 `inbox.max_active` 对应子命令专属参数 `--state-dir`、`--max-active`，不出现在 `agent` 的参数中。`max_active: 0` 使用默认 4，负值报错。共用的 `agent` 执行设置和 `--timeout` 仍适用；`inbox` 默认不限时，不覆盖 YAML 或 CLI 显式 timeout，显式 `0` 表示不限时。

`inbox` 目前只支持本地宿主，transport 使用 `local` 或没有 server URL 的 `auto`；不能与 `--server-url`、web 或 stdio transport 组合。恢复使用状态目录，不接受 `agent` 的 JSONL `--resume`。`inbox` 配置属于宿主本地配置，不作为远程 Agent 配置下发。

状态目录相对宿主工作目录解析，默认 `.cyber/active`，包含 `active.sqlite`、WAL 文件和用于所有权锁的 `owner.sqlite`。目录只能由一个 inbox 宿主持有，退出或崩溃后释放锁；再次使用该目录恢复任务历史、未处理输入、结果 outbox 与来源游标。应把整个目录作为一个持久状态单元管理，不单独复制正在写入的数据库文件。

`inbox` 的 IOA 身份由 hostname 与绝对状态目录派生；保持二者不变才能在同一 IOA 服务上恢复原地址。更换目录或主机名会使用另一绑定。任务共享宿主工作目录，历史隔离不提供文件隔离。操作说明见 [Inbox 常驻路由](agent.md#inbox-常驻路由)。

## 协作连接配置

IOA 客户端与服务端使用独立的扩展配置。在包含这些扩展的发行版中，可合并到 `cyber.yaml`：

```yaml
node:
  name: worker
ioa.client:
  url: http://127.0.0.1:8765
  token: server-access-key
  space: team
ioa.server:
  url: http://127.0.0.1:8765
  token: server-access-key
```

客户端 `url` 决定协作服务，`token` 是注册所用的服务凭据，`space` 默认为 `default`；未配置 URL 时，Web 节点从 `--server-url` 推导 `/ioa` 连接；没有 Web 地址时使用产品安装的进程内客户端。服务端配置独立 HTTP 服务的监听 URL 与 access key。Web 中的浏览器桥接由 Hub 宿主装配，操作见[Web 与协作](user/web.md#ioa-消息空间)。

`--ioa-url`、`--server-token`、`--space` 覆盖对应命令作用域的配置，`ioa serve --addr/--token` 设置独立服务。身份只配置在 `node.name`，连接设置分别放在 `ioa.client` 与 `ioa.server`。服务端不使用客户端的 space。

字段缺失与显式空字符串不同：客户端 `url: ""` 禁用自动推导的外部连接。密钥与 URL 凭据在配置视图中脱敏，保存留空密钥保留已有值；连接或策略变更按候选 Profile 校验与替换，不迁移旧会话。IOA 地址和 Web 节点的 `--server-url` 分别配置。

## JEV / Reflex

JEV 通过自然语言 Claim 学习可复用能力，后台编译 Agent 持续修复 Reflex，
通过原生契约、真实轨迹回放和独立语义验证后才接管执行。

```yaml
jev:
  base_url: https://api.typesafe.ai  # 可替换为自有网关，追加 /v1/systemone
  api_key: ""  # 或 TYPESAFE_API_KEY
  model: jev-1.13.0
  timeout: 10s
  mode: auto
  learning: auto
  compilation_timeout: 0
```

`jev.learning` 可选 `auto`（学习及持续编译）或 `frozen`（只复用已有合格 Reflex）。
`jev.compilation_timeout` 默认 `0`，后台编译不设总时间限制；可显式设置正 duration。
每次 JEV 请求仍受 `timeout` 限制。编译失败返回具体诊断继续修复；取消或服务不可用
保留候选，真实证据不足等待新的任务证据。验证流程与复用边界见
[JEV / Reflex 机制](architecture.md#jev-与-reflex)。

## 工具审批配置

在包含 Guardrail 的发行版中，将以下内容合并到 `cyber.yaml`，并通过 `TYPESAFE_API_KEY` 或 `jev.api_key` 提供凭据：

```yaml
guardrail:
  provider: jev
  mode: auto
  review_timeout: 5m
  jev:
    level: standard
    on_error: block
    # criteria:
    #   review: "Require review for any active production probe."
```

`jev` 提供共享凭据、模型和请求超时，`jev.mode` 控制 Reflex 加速；Guardrail 独立选择策略，将 `jev.mode` 设为 `off` 后仍可执行工具检查。`guardrail.provider` 默认 `none`，仅配置凭据不会启用策略。

### 裁判来源

| `provider` | 判定方式 | 额外依赖 |
| --- | --- | --- |
| `none`（默认） | 不判定，所有调用直接执行 | 无 |
| `jev` | JEV 两阶段判定：首判；`auto` 模式对被标记调用再做后果复核 | `jev` 的凭据（`TYPESAFE_API_KEY` 或 `jev.api_key`） |
| `llm` | OpenAI 兼容 `/chat/completions` 裁判；`auto` 模式对被标记调用再做一次后果判定 | `guardrail.llm.base_url`、`guardrail.llm.model`，可选 `api_key` |
| `hybrid` | 小模型先判；仅**明确的 `REVIEW`**（高置信异常输出）才交 JEV 复核（JEV `record`→执行、`block`→拒绝、仍 `review`→人工）；低置信/不确定的 LLM 结果按 `ALLOW` 直接放行，不触发 JEV | LLM 与 JEV 两套配置；JEV 凭据仍由 `jev` 提供 |

`provider=llm` 不加载 JEV 客户端，也不需要 JEV 凭据；`provider=hybrid` 必须同时具备 LLM 配置与 JEV 凭据。**JEV 采用 harness 原生实现，本 fork 未修改其判定与两阶段逻辑**；自定义扩展集中在 LLM 审批裁判（`policy_llm.go`），hybrid 仅通过既有 `Policy` 接口调用 JEV。

LLM 裁判请求固定 `temperature: 0`、`max_tokens: 128`、非流式；系统提示词只要求输出 `ALLOW` 或 `BLOCK: 理由` 两档，并带有强 `ALLOW` 偏置：**默认允许；无法从调用本身确定后果时一律 `ALLOW`（record）**；只有明确会破坏可用性、数据完整性或系统配置且难以恢复时才 `BLOCK`。提示词不再要求 `REVIEW`，只读敏感文件（含渗透场景读取 `/etc/shadow`）不算 `block`。解析仍兼容 `REVIEW`（属于异常模型输出）以及大小写、代码围栏、额外文本、JSON 形式（含可选 `confidence`）。

```yaml
guardrail:
  provider: llm
  mode: auto
  on_error: review        # allow | review（默认）| block
  timeout: 3s
  concurrency: 8
  llm:
    base_url: https://gateway.example/v1
    model: judge-small
    api_key: ""
    temperature: 0
    max_tokens: 128
```

```yaml
guardrail:
  provider: hybrid
  mode: safe
  scope: [bash, subagent]      # 只判定这些工具；空=全部
  allow_prefixes: ["ls ", "cat "]  # 仅 safe 模式生效
  on_error: review
  llm:
    base_url: https://gateway.example/v1
    model: judge-small
  jev:
    level: standard
    on_error: block
```

### LLM 判定语义（没把握就允许）

| 模型输出 / 情形 | 决策 | 说明 |
| --- | --- | --- |
| `ALLOW`、无法确定后果、或明确表达不确定 | `record`（放行） | 提示词要求不确定时直接 `ALLOW` |
| 显式 `confidence` < 0.7（无论伴随 `ALLOW`/`REVIEW`/`BLOCK`） | `record`（放行） | 低置信 = 不确定，不得升级为 `review`/`block` |
| `BLOCK: 理由`（未声明低置信） | `block` | 明确破坏可用性/数据完整性/系统配置 |
| `REVIEW` 且无 `confidence` 或 `confidence` < 0.7 | `record`（放行） | 提示词不再要求 `REVIEW`，视为不确定 |
| `REVIEW` 且 `confidence` ≥ 0.7 | `review`（异常输出） | 兼容保留；`auto` 模式拦截并记录，`safe` 模式进人工审批 |
| 传输失败、超时、非 200、无法解析 | 按 `on_error` | 默认 `review`，可显式 `allow`/`block`；**失败不等于“不确定”** |

即：**判断不确定 = ALLOW；裁判调用失败/超时 = `on_error`（默认 `review`，不会静默放行）**。

### 配置项

| 配置项（`guardrail` 下） | 默认值 | 含义 |
| --- | --- | --- |
| `provider` | `none` | `none`、`jev`、`llm` 或 `hybrid`，见上表 |
| `mode` | `auto` | 自动判断后果；`safe` 等待人工审批 |
| `review_timeout` | `5m` | 人工审批最长等待时间，须为正的 duration |
| `scope` | 空（全部工具） | 只判定列出的工具名；空表示全部判定。见下方绕过面说明 |
| `allow_prefixes` | 空 | 仅 `mode=safe` 生效；命令文本按字面前缀命中时直接放行，不进入人工审批 |
| `on_error` | `review` | 仅 llm/hybrid：裁判**调用失败**（超时/网络/非 200/无法解析）时 `allow` 放行、`review` 进入人工判定、`block` 拒绝；模型“判断不确定”按 `ALLOW`，不算失败 |
| `timeout` | `3s` | LLM 单次请求超时，须为正的 duration |
| `concurrency` | `8` | LLM 最大并发请求数（1–1024），超出者按 `on_error` 处理 |
| `shadow` | `false` | 只判定并记录 AOP 决策事件，永远放行；用于灰度统计 |
| `jev.level` | `standard` | 风险预设：`permissive`、`standard`、`strict` |
| `jev.on_error` | `block` | 首次检查失败时使用 `review` 或 `block` 判断；后果判断失败始终拒绝 |
| `jev.criteria` | 对应 level 的预设 | 按 `record`、`review`、`block` 覆盖首次检查标准，值须为非空文本 |
| `llm.base_url` | 必填 | OpenAI 兼容端点；可写完整 `/chat/completions` 端点，否则自动补路径 |
| `llm.model` | 必填 | 裁判模型名 |
| `llm.api_key` | 空 | 可选 Bearer token；本地网关可不填 |
| `llm.temperature` | `0` | 采样温度（0–2） |
| `llm.max_tokens` | `128` | 输出上限（1–4096） |

所有新增字段在配置解析时校验，非法值会以中文原因让启动失败；例如 `allow_prefixes` 必须配合 `mode=safe`，`on_error` 只能用于 llm/hybrid。

`standard` 将本地分析和授权的低频探测归为 `record`，目标修改、高强度或未知影响归为 `review`，明确破坏、泄露或伤害归为 `block`；`strict` 对主动探测要求复核，对目标修改或未知影响使用 `block`。实际处理由 `mode` 决定，首次风险标准不会预先决定第二阶段的后果判断。

供 Saker 把裁判成本与指挥官/执行者成本分账：

- JEV 响应携带 token usage 时，以 `Emitter=guardrail` 上报真实用量；该取值保持兼容旧控制面。
- LLM 裁判（OpenAI 兼容 `/chat/completions`）的响应携带 `usage` 时，以 `Emitter=guardrail-llm` 上报。
- `provider=hybrid` 下 LLM 阶段发 `guardrail-llm`、JEV 阶段发 `guardrail`，两次判定各发一条事件、互不合并；仅在真正升级到 JEV 时才出现 JEV 事件。

两类事件都归入裁判（guardrail）分类，账本按 `Emitter` 再区分 JEV 与小模型裁判。响应缺少 usage、判定失败（超时、网络错误、非 200、无法解析）或调用取消时不发 usage 事件；缺失的计数字段不估算、不伪造（`prompt_tokens`/`completion_tokens`/`total_tokens` 只取供应商上报值，`cache_read` 等可选字段仅在存在时携带）。

LLM 裁判同样只接收脱敏后的本次工具名、参数和工作目录，不发送对话历史；常见凭据会被遮盖，但任意命名或编码的秘密不保证被识别。

### 风险与绕过面

- **fail-open**：`on_error: allow` 在裁判**调用失败**时放行工具调用，加载时会打 WARN。只应在能接受该风险的灰度环境使用；生产默认 `review`，需要严格失败关闭时用 `block`。注意与“判断不确定”区分：不确定的 LLM 结果本来就按 `ALLOW` 记录，不经过 `on_error`；只有传输/解析失败才触发 `on_error`。
- **shadow**：`shadow: true` 时不会产生人工审批、也不会拒绝调用，只在 AOP 中记录判定结果。适合先观察 LLM 误判率，再切到 `review`/`auto`；shadow 不覆盖调用取消，取消仍会终止调用。
- **scope 只按工具名过滤**：它不是安全边界。`bash` 可以执行任意命令、`subagent` 可以驱动子会话、MCP 或命令适配层可能按新名称暴露工具；使用未列入的工具即可绕过判定。`scope` 用于降低判定成本、减少审批噪音，不能替代宿主权限、沙箱或网络边界。未命中的工具不产生判定事件。
- **allow_prefixes 只是字面前缀**：不做语义分析。`ls` 会命中 `lsof`；需要边界时写带空格的 `"ls "`。命令拼接（例如 `ls; rm -rf /`）仍以 `ls ` 开头。只应放入确定低风险、无副作用的前缀，并与 `mode=safe` 一起使用。

Web 保存模式可更新当前运行时，仅影响后续调用；首次启用、策略或凭据变更需要验证并替换 Profile。密钥留空保留已保存值，或使用服务端 `TYPESAFE_API_KEY`。日常审批操作见[工具指南](user/tools.md#工具准入与审批)。

## 数据与 Web

数据目录优先级为 `--data-dir`、`CYBER_DATA_DIR`、配置中的 `misc.data_dir`；未指定时，依次复用当前目录已有 `.cyber`、二进制旁已有 `.cyber`，否则使用 `~/.cyber`。复用旧目录时提示路径，不自动迁移历史和缓存。

配置内的相对 `data_dir` 相对于定义它的文件；CLI 和环境变量中的相对路径相对于工作目录。配置查看与目录发现不创建目录。

Web 设置沿用现有配置接口。保存目标为显式 `-c`、当前目录中已加载的配置，最后才是用户配置。页面展示可编辑的文件配置；运行时启动参数、环境变量可能覆盖这些值。

**node 模式的 LLM 配置来自远端 server。** 使用 `cyber-scan agent --server-url ...`（自动或 `web` transport）时，node 先连接 server，收到下发配置后才初始化模型和执行启动任务。node 本地配置文件、模型 CLI 参数、LLM 环境变量及编译默认值不参与模型选择，也不会覆盖远端值；远端没有模型配置时不回退到本地模型。server 地址、节点身份和本地数据目录仍在 node 本地配置。

node 上线、重连和 Web 配置保存后，server 自动下发实际生效的 LLM 配置，包括 server 配置文件、环境变量和启动参数解析后的 API key、模型与端点。node 无需单独配置相同的 key；运行时凭据只用于下发和连接检测，不回写配置文件。Web 顶部的 LLM 健康提示同样检测 server 实际生效的配置。

保存只修改目标层中编辑过的值，保留未修改的继承项和空白密钥，不把用户层或运行时凭据复制到项目文件。继承的 profile 要到其来源文件删除。保存前按原分层构建候选运行时，验证失败保留旧文件和旧运行时。
