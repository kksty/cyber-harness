# 工具

[使用者指南](../README.md#使用者指南) · 前一篇：[会话与上下文](sessions.md) · 下一篇：[Skills 与知识](knowledge.md)

工具是 Agent 读取和改变环境的方式。模型看到工具名称、说明和参数结构，再请求执行；cyber-scan 返回实际结果，模型据此继续工作。可用集合取决于发行版及已安装扩展，运行时可用 `/status` 查看。

## 文件与命令

文件工具支持读取、查找和修改内容，`bash` 支持命令式工作。工作目录决定相对路径的起点，宿主权限决定进程实际可以访问的环境。会话本身不提供容器或文件系统隔离。

`bash` 既可以执行宿主上的程序，也可以调用 cyber-scan 注册的命令。例如 `gogo`、`spray` 和 `scan` 可以直接调用内置扫描器，无需在 PATH 中另放一个同名程序；普通外部命令仍要求系统安装对应工具。

交互模式的 `!` 前缀直接执行命令，适合查看帮助或手动完成明确的操作：

```text
cyber-scan> !gogo -h
cyber-scan> !tmux ls
```

内置命令和外部命令可以通过执行适配层组成管道。不同命令有自己的参数作用域，例如 Agent 的 `-p` 表示 prompt，gogo 的 `-p` 表示端口。参数细节见[命令参考](../reference.md)。

## 工具准入与审批

Guardrail 在工具执行前检查本次调用的参数和工作目录。先按[工具审批配置](../configuration.md#工具审批配置)启用 JEV 策略；仅设置 API key 不会开启检查。

| 模式 | 被标记的调用如何处理 |
| --- | --- |
| `auto`（默认） | 再次判断实际后果；明确无害时执行，后果不确定或有害时返回工具错误，Agent 继续工作 |
| `safe` | 等待人工批准或拒绝；首次判断为 `review` 或 `block` 的调用均进入审批 |

首次判断为 `record` 的调用直接继续。每次调用都会重新检查，既有批准只适用于一次调用。交互 CLI 会提示待审批操作，可用：

```text
/guardrail pending
/guardrail approve <operation-id>
/guardrail reject <operation-id>
/guardrail pending --session <session-id>
```

默认范围是当前会话及其仍存活的子会话，`--session` 可选择其他存活会话。批准使原调用执行一次；拒绝、审批超时或取消返回工具错误。非交互运行需通过控制客户端处理审批，或等待超时。

Web 在工具所属轮次内显示审批卡片与处理结果，侧栏标记需要处理的会话。断线时审批按钮禁用，重连后重新查询运行时状态；历史卡片保留记录，重启不会恢复可执行审批。批准记录与工具执行结果分别展示，批准本身不表示执行成功。

顶部 Guardrail 菜单可切换自动或人工模式；Settings → Guardrail 设置策略与凭据。模式切换仅影响新调用，已有审批继续等待。凭据和策略变更会重新构建 Profile，并取消原有执行与待审批操作。连接测试向 JEV 发送本地只读操作的描述，不运行工具，降级结果不算连接成功。

Guardrail 是工具调用的检查边界。已启动子进程内部的操作和原始 PTY 输入仍受宿主权限与环境控制。JEV 会收到脱敏后的工具参数和调用上下文；常见凭据会被遮盖，但任意命名或编码的秘密不保证被识别。运行前应按实际数据范围配置凭据、网络和执行环境。

## 裁判来源、范围与灰度

Guardrail 的判定来源可配置为 `none`、`jev`、`llm` 或 `hybrid`，配置方法见[工具审批配置](../configuration.md#工具审批配置)。

| `provider` | 被标记调用的处理 |
| --- | --- |
| `none` | 不检查，直接执行 |
| `jev` | JEV 首判；`auto` 模式再判定实际后果，`safe` 模式进入人工审批 |
| `llm` | OpenAI 兼容裁判首判；`auto` 模式再判定实际后果，`safe` 模式进入人工审批；裁判不可用时按 `on_error`（默认 `review`）处理 |
| `hybrid` | 小模型先判，**仅明确的高置信 `REVIEW`** 才交 JEV 复核（JEV `record` 直接执行、`block` 拒绝、`review` 进入人工审批）；低置信/不确定的 LLM 结果按 `ALLOW` 放行，不触发 JEV |

`llm` 不依赖 JEV 凭据；`hybrid` 需要同时配置 LLM 与 JEV。LLM 只收到本次调用的工具名、参数和工作目录（常见凭据会脱敏），不会收到对话历史。LLM 裁判遵循“没把握就允许”：判断不确定（含低置信/缺 confidence）按 `ALLOW` 记录；**不确定不等于失败**，只有传输失败/超时/无法解析才按 `on_error` 处理（默认 `review`，可显式 `allow`/`block`）。

```yaml
extensions:
  guardrail:
    provider: hybrid
    mode: safe
    scope: [bash, subagent]
    allow_prefixes: ["ls ", "cat "]
    on_error: review
    llm:
      base_url: https://gateway.example/v1
      model: judge-small
```

以下配置能降低误报干扰和判定成本：

- `scope: [bash, subagent]` 只判定列出的工具名，空表示全部。**它只按工具名过滤，不是安全边界**：未列入的工具（或其他入口、命令适配层、子会话）仍可能执行同样的操作；`bash` 内部也可以执行任意命令。scope 用于减少调用量和审批噪音，不能替代宿主权限与环境隔离。
- `allow_prefixes: ["ls ", "cat "]` 只在 `mode=safe` 生效，命令文本按**字面前缀**命中时直接放行，不进入人工审批。它不做语义分析：`ls` 会命中 `lsof`，需要边界时写带空格的 `"ls "`；命令拼接（`ls; rm -rf /`）同样以 `ls ` 开头。只应放入确定低风险的前缀。
- `on_error: allow` 是显式 **fail-open**：裁判超时、网络失败或返回无法解析时放行调用，加载时会打印 WARN。生产建议保持默认 `review`，或在严格场景使用 `block`。
- `shadow: true` 只判定并记录，永远放行且不创建审批。适合先用灰度观察 LLM/hybrid 的误判率，再切回 `auto` 或 `safe`；取消调用仍会终止执行。

JEV 响应携带 usage 时，Guardrail 会把真实 token 用量以 AOP usage 事件上报；响应没有 usage 时不会估算或伪造。

## 后台任务

较长的命令可以在等待一段时间后转入后台，使 Agent 继续处理其他工作。以下是模型调用 `bash` 时的参数形态：

```json
{"command":"某个长时间运行的命令","wait":5,"timeout":120}
```

`wait` 表示此次工具调用在前台等待的秒数。5 秒后尚未结束，工具返回后台 session ID，命令继续运行；`wait: 0` 或省略则等待完成。`timeout` 是命令的总运行期限，转入后台后仍生效；省略使用默认 600 秒，显式设为 0 表示不设置总期限。

普通前台命令的 stdin 默认是 EOF，`read`、`cat` 等读取输入的命令不会空等到超时。需要后续输入时，显式设置 `wait > 0`，再用 `tmux send-keys` 发送内容；`tmux new-session` 的交互会话同样保留输入。`wait=0` 被 Inbox 中断而转后台时仍保持原来的 EOF。Go 调用方显式提供的 `Options.Stdin` 优先使用其 reader。

完成结果会送回 Agent 会话，运行期间也可通过 `tmux` 查看输出、发送交互输入或停止任务。这是 cyber-scan 自己的工作管理命令，不要求系统安装 tmux。

```text
cyber-scan> !tmux ls
cyber-scan> !tmux kill -t <session-id>
```

长输出可能只在模型上下文中保留部分内容。需要查看较早输出时使用工具返回的读取提示，完整管理语法见[终端命令说明](../../exts/skills/assets/runtime/tmux.md)。远程 API 的前台执行入口有独立契约，不能把 Agent 的后台等待参数等同于所有入口的行为。

## 网络与代理

cyber-scan 的工具网络出口由本地代理 Hub 管理。`--proxy` 设置初始上游，Agent 可以通过 `proxy` 命令管理和切换节点；`mitm` 用于查看捕获的请求和响应。稳定的本地地址使工具可以复用路由配置。

```sh
cyber-scan scan -i http://127.0.0.1:3000 --verify=off --proxy socks5://127.0.0.1:1080
```

内置客户端接收代理配置，外部程序通过代理与 CA 环境变量接入。忽略这些变量的程序不一定经过 Hub；上游切换也不会把已经建立的 TCP 连接迁移到另一出口。

HTTPS 捕获依赖客户端信任 Hub CA。`--mitm=false` 关闭捕获，保留路由。模型 API 使用独立的 `--llm-proxy`，与扫描工具的出口分别配置。协议、节点管理和捕获限制见[代理参考](../reference.md#代理proxy)。

## 浏览器与外部工具

cyber-scan 的浏览器支持页面导航、交互、截图和网络捕获。持久浏览器会话保留多步交互状态，适合需要登录或 JavaScript 渲染的页面；它的状态与 Agent 的对话历史不同，恢复聊天记录不会恢复旧浏览器实例。具体命令见[浏览器说明](../../exts/skills/assets/runtime/playwright.md)。

Arsenal 管理外部安全工具，并将安装目录加入子进程 PATH。安装后的工具仍需要满足平台与运行依赖。原生桌面录屏是另外的可选构建，见 [record](../record.md)。

## 搜索与资源

`web_search` 获取网络搜索结果，`fetch` 读取 URL 内容。搜索优先使用支持该能力的 Provider，必要时使用已配置的 Tavily；没有可用后端时会失败。搜索结果提供线索，具体判断仍需读取和核实来源。

所有搜索后端统一通过已有的 Function calling 工具 `web_search` 暴露，参数均为 `query` 和可选的 `num`。DeepSeek、OpenAI、Anthropic、Tavily 和 DuckDuckGo 的选择及失败后备由搜索扩展和工具内部处理，用户与模型无需切换工具名，也不会额外注册 `deepseek_search` 或 `deepseek_web_search`。

搜索后端由 `exts/search` 扩展负责，包括独立的 HTTP 客户端、请求、响应解析和资源关闭；`agent/provider` 只提供已有模型配置，不再声明或执行搜索能力。DeepSeek 使用独立的 [Search API](https://api-docs.deepseek.com/zh-cn/guides/search)，返回标题、链接及每条结果的摘要。使用官方 `api.deepseek.com` 时，OpenAI 和 Anthropic 协议均会自动接入，复用当前 Profile 的 API key、代理和超时，不需要额外搜索配置，也不受聊天模型名称限制。自定义网关只有显式配置 `provider: deepseek` 才会使用这一接口，并要求网关提供对应的 `/v1/tools/search` 路由；仅使用 DeepSeek 模型名称不会启用。

`web_search` 的 `query` 在 DeepSeek 上限为 128 个 Unicode 字符，支持 `site:`、`-site:`、引号、`lang:`、`after:` 和 `before:` 等搜索语法；`num` 仍为 1–10，默认 5。搜索无结果会正常返回；错误时沿用现有后备搜索流程，后续调用仍可重试。Search 按成功且非空的请求次数单独计费，详情见官方文档。

设置环境变量 `DEEPSEEK_API_KEY` 后，可在仓库根目录运行 `go test ./exts/search -run '^TestLiveDeepSeekWebSearchTool$' -count=1 -v`，加载搜索扩展并通过注册的 `web_search` 工具依次验证官方 DeepSeek、OpenAI 和 Anthropic 配置。测试记录官方接口响应，核对来源与摘要，并在接口失败时取消请求、阻止后备搜索，避免把其它搜索源的成功误判为 DeepSeek 成功；未设置密钥时跳过，接口失败时停止后续请求。`HTTP 402 / Insufficient Balance` 表示账号余额不足，需充值后再验证。

Cyberhub 面向已经加载的指纹和 POC 资源，支持按指纹、CVE 等查询关联。它与公共网络搜索的用途不同。资源配置及扫描器用法见[参考手册](../reference.md#cyberhub-资源)。
