# Behavior and data / 运行与数据

## Notifications

Codex and Claude Code connect independently and share the configured channels.
Overview and Settings show each assistant separately. The header and Activity
identify the source; old records without a source remain Codex records.

### Codex

| Type | Codex event | Meaning |
| --- | --- | --- |
| Turn ended / 本轮已结束 | `Stop`, with local `task_complete` events as a fallback | This turn produced its final output; not a guarantee of task success. |
| Answer needed / 需要回答 | `PreToolUse` for `request_user_input` or `request_user_input_async` | A structured question needs your input. |
| Approval needed / 需要授权 | `PermissionRequest` | Codex is requesting approval for an operation. |

Nudilo requires a Codex version supporting `hooks/list`, `config/batchWrite`
and these events. Actual hook execution and completion delivery have been
validated locally with `codex-cli 0.166.0-alpha.6`, an isolated Codex home,
a loopback model fixture and a fake notification sender. Unsupported versions
show an upgrade error.

Disconnect Codex before changing its configuration folder in Advanced settings;
this removes the old listeners before another folder is associated.

Codex chats already open when Nudilo installs its listeners can retain their
earlier hook configuration. To cover turn endings in those chats, Nudilo watches
new records in local `sessions/rollout-*.jsonl` files (including date subfolders).
It starts existing files at EOF, ignores earlier timestamps in newly discovered
files, and deduplicates completion events against Hooks by turn ID. Historical
completions and sub-agent rollouts are not replayed. Nothing is written to Codex
transcripts, and no Codex restart is required for these completion reminders.

This fallback recognizes only explicit `task_complete` events with a turn ID;
it does not infer completion, questions or approvals from conversation text.
The [transcript format is not a stable Codex interface](https://learn.chatgpt.com/docs/hooks#common-input-fields),
so compatibility needs checking when Codex changes it. Reopen existing chats
to load Hooks for answer and approval reminders. The UI says “Listening for
Codex” until a new event is actually received; a successful channel test alone
does not verify automatic event delivery.

Natural-language questions without a structured tool request are treated as
turn endings. Client-specific interactions not exposed through these Hooks
cannot be guaranteed. Return to the originating assistant on your computer to answer or approve.

<a id="claude-code"></a>

### Claude Code

Install and open Claude Code first. Nudilo detects its local executable and
configuration directory (`CLAUDE_CONFIG_DIR`, otherwise `~/.claude`; on Windows,
`%USERPROFILE%\.claude`). Advanced settings accepts a custom folder or executable.
It runs only `--version`, never a model turn. Minimum versions are **2.1.78** on
macOS/Linux and **2.1.295** on Windows; newer versions can keep notification-hook
failures non-blocking on Windows even without Git Bash. Reopen existing sessions
if they do not load the listeners. This covers local Claude Code sessions using
that configuration, not the ordinary Claude chat app or a remote/cloud runtime.

| Claude Code event | Nudilo reminder |
| --- | --- |
| `Stop` | Turn ended |
| `PreToolUse` for `AskUserQuestion` | Answer needed |
| `PreToolUse` for `ExitPlanMode` | Approval needed |
| `PermissionRequest` for other tools | Approval needed |
| `Elicitation` | Answer needed for MCP input |
| `StopFailure` | Interrupted, separate from a completed turn |

Only main-conversation events are accepted. Subagent completions, idle notices,
manual interruption and ordinary tool errors do not generate reminders. Sandbox
network permission dialogs are not covered by this adapter. A natural-language
question without an input tool is a turn ending. Other Stop hooks may continue
work after Stop fires: a turn-ending reminder is not proof of task success.

Connection merges seven listeners into user `settings.json`, retaining other
hooks and settings; original bytes are backed up under Nudilo's `backups` folder.
JSONC comments remain in that backup; merged output is standard JSON. A local
receipt records only the folder and owned command. Disconnect removes that exact
command, preserves foreign handlers even in shared groups, and disables cached
Nudilo helpers for Claude Code. Codex and the shared helper remain available.
Invalid files and detectable hook restrictions are preserved, with an error
instead of silently rewriting them. Project and remotely managed policies still
apply, including policies not visible to the desktop app.

Refresh checks the version, settings and helper without modifying them.
Reconnect repairs missing or changed Nudilo listeners. Status stays “Listening
for Claude Code” until a new event arrives after connection; an IM test alone
does not confirm event collection. The integration is tested with an installed
compiled helper, realistic event fixtures and an in-memory sender; no live Claude
model session or real IM delivery was performed for this addition.

The helper reads bounded stdin, queues minimal events, exits successfully and
prints nothing. It never answers questions, approves operations or adds model
context. It does not read Claude transcripts. An optional summary includes final
reply text, non-secret question text or a permission description; MCP elicitation
content, plan details and API error details are excluded. Summary remains off by
default. Prompt IDs distinguish repeated turns; older runtimes use opaque tokens
from `SessionStart` and `UserPromptSubmit`, stored in `claude-turns/` without
prompts or commands. On session start, tokens older than 30 days are removed and
the remaining set is capped at 1,000. Event identities include the assistant.

See the official [hooks reference](https://code.claude.com/docs/en/hooks) and
[settings documentation](https://code.claude.com/docs/en/settings).

## Desktop feedback and activity

Closing the main window keeps Nudilo in the system tray and notifications running.
On macOS, this also hides the Dock icon; reopening the window restores it. Launch
at login stays in the tray without a Dock icon. If the tray cannot be created,
the window stays accessible. Quit from the menu exits Nudilo.

Overview shows every channel's mark and a separate configuration status. A saved
destination does not count as configured if its local credentials are missing.
The reminder control animates only when toggled and respects reduced motion.

Saved channels open to a summary, not an empty setup form. Editing restores the
local values automatically and keeps secrets masked. Cancel discards the draft;
saving returns to the summary even if a subsequent test fails. Recipient entry
is handled by pairing only; changing application credentials invalidates the
old draft binding until it is paired again. Existing saved recipients survive
an upgrade. Editing never writes credentials until Save is selected.

Local validation is shown below its input and focuses the first invalid field.
Operation feedback appears near the bottom of the window, dismisses automatically
(success after 3 seconds, errors after 8), and can be closed. Hover/focus pauses
its timeout. Page, channel and destination changes clear it; late results from
an earlier page do not appear on the new page. Delivery errors remain in Activity.

Refresh status reads helper availability, Codex policy and trusted event hooks;
it does not reinstall or grant trust. Reconnect performs installation/repair.
Refreshing preserves the existing chat monitor and never starts a model turn.
A verified registration still waits for a new real event to confirm delivery.

Clear all in Activity requires confirmation. It removes visible history and
notification content and cancels pending retries. A send already underway can
finish before clearing; subsequent new events continue normally. Up to 500
minimal event IDs, assistant names, turn IDs, kinds and timestamps remain for deduplication and
connection status, including after restart. Test records are removed completely.
A failed disk write preserves the visible history.

## Delivery

Notification content follows the current app language, including system language
matching. Optional title and four event-body templates are stored per language
in `settings.json`; absent fields use localized defaults. The title always
includes the originating assistant and the event type. All five IM adapters use the same renderer.
The editor previews that renderer using sample data and offers per-language
save, cancel and restore-default actions. See [customization](CHANNELS.md#content).

Only `{agent}`, `{project}`, `{time}` and `{summary}` are expanded, once, in the body.
Project names and summaries remain literal, without translation or recursive
expansion. Summaries require the existing opt-in, including during retries.
Titles are limited to 80 Unicode characters on one line; each body template is
limited to 1,000. Expanded messages have a shared 3,000-byte UTF-8 budget and
truncate on character boundaries, keeping the header. The preview shows the
same truncation for its example data. Invalid stored fields fall back to defaults;
invalid edits cannot be saved. Empty fields restore the localized default.

Each send attempt resolves the current preferences, so queued retries follow
language/content changes. Rendered messages are transient and are never written
to the inbox or history; history retains event data under the existing summary
policy. Connection tests use the configured title and a localized fixed test
body and a `Codex / Claude Code` header. Customizing text does not send a test or contact any external platform.

Events are persisted before delivery and deduplicated within each assistant by turn and event
identity. Only failed channels are retried. While the same app process stays
running, network failures and rate limits receive up to three automatic attempts,
then can be retried from Activity. A timeout between platform acceptance and local
bookkeeping can still result in a duplicate.

Events generated during a recorded pause, received with their assistant disconnected or type disabled, or before any channel
is configured are recorded as skipped. They are not replayed on resume.
Pending events expire after 24 hours. The app cannot send while exited or the
computer is asleep.

Each process launch starts a new notification window. Queued events created
before that launch and pending deliveries saved by the previous process are
marked **Skipped**, with a reason in Activity, rather than automatically replayed.
This also applies to later queue batches and delayed writes of older events.
New events created after launch are delivered normally, even before the first
worker pass. Previously sent and failed deliveries keep their status; an explicit
retry of a failed delivery remains available within the 24-hour limit. The normal
history retention limit still applies to skipped records.

Closing the window keeps the app running in the tray and does not reset delivery.
Waking the same running process also retains its pending retries, subject to
pause rules and expiration. Quitting, restarting or applying an update starts a
new process and never automatically replays its predecessor's pending deliveries.

Each platform can send to a personal bot conversation or a group/channel.
Saving a different destination cancels pending retries for the old one, so
historical messages are not forwarded to the new recipient. Other-mode setup
is retained in the local credential file. See [channel setup](CHANNELS.md).

“Delivered to platform” means the platform API accepted the message. Phone
notifications still depend on app permissions, mute settings and connectivity.

## Updates

Automatic updates are on by default: check after 30 seconds, then every six
hours, download verified stable releases in the background, and wait for the
user to restart. Manual checks remain available when automatic updates are off.
Read-only and system-package installations check versions but offer manual
downloads. Update requests contain no IM credentials or notification data.
See [automatic updates](UPDATES.md) ([中文](UPDATES.zh-CN.md)).

## Local data

The original CallBell identifiers are retained for compatible upgrades:

| OS | Data directory |
| --- | --- |
| macOS | `~/Library/Application Support/CallBell` |
| Windows | `%AppData%\CallBell` |
| Linux | `$XDG_CONFIG_HOME/CallBell` or `~/.config/CallBell` |

Settings → Open data folder opens this directory. The newest 500 records and
all pending deliveries from the current run are retained. Existing channel settings, credentials
and history are preserved when upgrading. Older settings default to system
appearance and language. `CALLBELL_DATA_DIR` is an override for isolated tests.

Credentials are encrypted with AES-GCM in `credentials.vault`; the random
32-byte key is stored beside it in `credentials.key`. Both files use `0600`
permissions on macOS/Linux and inherit the user-data directory access controls
on Windows. The protection boundary is the current OS user, not a separate
master password. Back up both files together; neither credentials nor the key
are copied into settings/history or release artifacts.

Nudilo reads and writes local credential files only. If local credentials are
missing, enter and save the configuration in Channels.
The completion fallback reads session metadata and appended local records;
only the same minimal notification fields enter the queue. Reply text is not
stored or sent unless summaries are enabled. Disconnect stops the monitor,
and cached helpers also honor the disconnected setting.
Private account pairing temporarily reads bot messages for at most three minutes.
Only a fresh private verification code selects the recipient. Receivers close on
success, cancellation, timeout or a channel/page change; incoming content and
codes are never persisted. The destination changes only after saving.

Linux does not require a Secret Service or `libsecret-1-0`. There is no telemetry.

Codex association backs up modified files under the app's `backups` folder,
merges the user's `hooks.json`, and trusts only this app's exact helper command.
Original JSONC comments remain in the backup; merged output is standard JSON.
Disconnect removes only Nudilo's listeners and retains other Hooks.
Unreferenced old trust hashes cannot execute commands. Disconnect, remove
channels and disable launch at login before uninstalling if you want to
remove the integration too.

## 中文说明

<a id="claude-code-zh"></a>

### Claude Code 关联

先安装并运行过 Claude Code，Nudilo 会自动识别本机程序及配置目录；两种助手各自关联、刷新和停止，共用已保存的通知渠道，私聊无需重新绑定。群机器人若启用关键词校验，请同时允许 `Codex` 和 `Claude Code`。

- macOS/Linux 需要 Claude Code 2.1.78+，Windows 需要 2.1.295+。版本检查只运行 `--version`，不会启动模型任务。Windows 的最低版本保证监听失败时可以明确继续原操作。
- 默认读取 `CLAUDE_CONFIG_DIR` 或 `~/.claude`（Windows 为 `%USERPROFILE%\.claude`）；高级设置支持指定配置目录及程序。已有会话未生效时请重新打开。本机配置对应的 Claude Code 会话可用，普通 Claude 聊天、云端或远程运行环境不自动接入。
- `Stop` 提醒本轮结束；`AskUserQuestion` 和 MCP `Elicitation` 提醒需要回答；`ExitPlanMode` 和其他工具的 `PermissionRequest` 提醒需要授权；`StopFailure` 单独提醒异常中断，不误报成功。
- 仅采集主会话。子代理、空闲提示、手动中断、普通工具报错不发送提醒；当前不覆盖沙箱网络权限弹窗。普通文本提问仍算本轮结束，其他 Stop hooks 也可能让任务继续，不能据此保证整个任务成功。
- 关联时备份原文件，再合并七类监听到 `settings.json`。其他配置和 hooks 保留，JSONC 注释保存在备份中。停止关联只移除 Nudilo 自己的命令，同时让已缓存的监听静默；不会停掉 Codex 或删除共用程序。配置损坏或本地可见策略禁止时会提示，不绕过限制。
- 状态在收到本次关联后的新事件前显示「正在监听 Claude Code」。发送测试只验证 IM，不能证明助手事件已打通；「刷新状态」只检查，「重新关联」才会修复配置。
- 监听程序不联网、不代答、不批准、不返回模型上下文，也不读取 Claude 会话全文。摘要默认关闭；开启后只采集可用的最终回复、非秘密问题或授权描述，不收集 MCP 请求内容、计划全文或 API 错误详情。
- 支持 prompt ID；旧版本使用 `claude-turns/` 中的随机轮次标记，既不存提示词也不存命令。每次会话启动清理超过 30 天的标记，并最多保留 1,000 份。去重、记录及连接状态按助手区分，旧记录仍视为 Codex。
- 此次已通过真实编译的监听程序、隔离事件样本和模拟 IM 验证，未启动真实 Claude 模型会话或进行真实 IM 投递。

### 通用行为

- 「设置 → 通知文案 → 自定义」支持统一标题和四类正文、变量插入、实时预览、取消修改及恢复默认。中英文分别保存；新发送和重试跟随软件当前语言，没有自定义内容时使用对应语言的默认文案。恢复默认后需保存，只影响当前语言。
- 正文只展开 `{agent}`、`{project}`、`{time}`、`{summary}`，项目名和摘要不翻译、不递归展开。摘要开关关闭时始终省略摘要；标题始终保留实际助手名称和提醒类型。标题最多 80 字符，每类正文最多 1,000 字符，展开后的消息按 3,000 个 UTF-8 字节安全截短。渲染结果不持久化；发送测试使用当前标题和语言，正文为固定的连接测试说明。详见[通知文案设置](CHANNELS.zh-CN.md#content)。

- 关闭主窗口后继续在托盘接收通知；macOS 同时隐藏 Dock 图标，重新打开窗口时恢复。登录启动时只驻留托盘；托盘不可用时会显示窗口，避免无法操作。通过菜单中的「退出」才会结束 Nudilo。

- 已配置的渠道展示概览，编辑时自动填入本地配置，密钥始终遮挡。取消修改不会保存草稿；保存成功后回到概览，即使随后的测试发送失败也不会丢失配置。私聊接收人通过绑定识别，更换应用凭据后需重新绑定，已有接收目标升级后仍可使用。

- 概览分别显示各渠道图标与配置状态；提醒开关带轻微过渡动画，并跟随系统减少动态效果设置。
- 必填或格式错误显示在输入框下方；操作结果使用可关闭的短暂浮层，切换页面、渠道或发送方式后会清除，晚到的异步结果不会串页。
- 「刷新状态」分别检查对应助手的可执行文件、配置策略和监听状态，不重装关联、不启动模型任务，也不重置已有会话监听。
- 「清空全部」需确认，会删除记录内容并取消待重试通知。已经在途的发送可先完成；保留最多 500 条最小事件标识与时间用于去重及关联状态，不保留通知正文。新事件继续正常发送。

- 凭据改为本地 AES-GCM 加密文件，密钥也在本机数据目录；macOS/Linux 文件权限为 `0600`，Windows 继承用户数据目录权限。备份时需同时保留 `credentials.vault` 和 `credentials.key`。
- 凭据只从本地文件读写，不访问系统钥匙串；本地凭据缺失时，在渠道页面重新填写并保存。

- 本轮结束只代表这一轮输出结束，不代表整个任务完成；回答和授权仍在电脑端处理。
- 安装前已打开的 Codex 会话可能保留旧监听配置。Nudilo 只读取本机会话新追加的明确结束事件，支持已有会话的结束提醒，不补发历史记录，并与 Hooks 按轮次去重。回答和授权提醒仍需重新打开已有会话。会话记录格式并非稳定接口，Codex 更新后需要验证兼容性。
- 连接测试成功不代表自动提醒已生效；收到新的真实事件前，界面显示「正在监听 Codex」。停止关联会同时关闭已有会话监听，并让缓存的事件助手停止写入。
- 更改 Codex 的配置目录前需先停止关联，移除旧目录中的监听，再关联新目录。
- 事件先写入本地，再按渠道发送。同一次运行中的临时失败最多自动尝试三次；平台确认前后发生超时，仍可能重复发送。
- 暂停、关闭提醒类型或未配置渠道时收到的事件不会补发；待发送通知超过 24 小时后过期。
- 启动前产生的队列事件及上次运行遗留的待发送记录会标为「已跳过」，保留原因并遵循现有历史保留数量限制；分批读取或延迟写入的旧事件也不会补发。启动后的新事件正常发送。已发送和失败的渠道保留原状态，失败记录仍可在 24 小时有效期内主动重试。
- 关闭窗口仍在托盘运行，通知照常发送；同一进程从睡眠中恢复时仍遵循暂停和过期规则继续处理。退出重开、重启或安装更新后的新进程不会自动补发上次积压。
- 每个平台支持私聊或群聊／频道。保存新的接收目标后，旧目标的待发通知不再转发；另一模式的配置保留在本地凭据文件中。详见[配置指南](CHANNELS.zh-CN.md)。
- 私聊绑定仅临时读取机器人消息，最长三分钟；结束后关闭连接，验证码和收到的聊天内容不会保存。识别出的账号在保存后才生效。
- 「已送达平台」不代表手机必然弹窗，还取决于手机权限、群聊免打扰及网络。
- 为兼容升级，数据目录与凭据标识保留 CallBell。旧设置自动采用跟随系统的语言和外观。
- 关联前会备份，停止关联仅移除 Nudilo 的监听。卸载前可先停止关联、移除渠道并关闭登录启动。


## Delivery health, pauses and project rules

See [Everyday use](USAGE.md) ([简体中文](USAGE.zh-CN.md)) for the UI workflow.
Health is derived from retained per-channel delivery records and their last attempt
timestamps. `channel_since` scopes it to the last successful configuration save;
tests and real events keep separate last-success times. It is evidence of platform
acceptance, not a mobile read receipt. Diagnostics only check local credentials,
read-only listener registration and disposable write probes in the data and inbox
folders; they do not send notifications or repair configuration.

Timed pauses persist `pause_started` and `pause_until`. The worker checks the
deadline before ingestion and persists its resume before sending. The latest pause
interval remains after resuming so queued events from that interval stay skipped.
Changing a pause duration preserves its start. Pending deliveries from before the
pause remain held within the same running process and expire normally; a process
restart skips them under the startup policy above. Storage failures leave the pause active;
the worker retries the resume on subsequent passes.

Project rules use exact existing project names, with assistant-specific rules
before all-assistant rules. The helper does not collect full paths for routing.
Without a match, all enabled channels receive new events. Matched rules never
fall back to another destination. The worker rechecks rules before each pending
attempt, skipping channels removed from the rule without adding new destinations
to existing records. Removing a rule affects future events; previously skipped
notifications do not become pending again. Explicit tests bypass routing.

Activity search combines with assistant and per-channel status filters. Dates
use the desktop's local timezone. The initial view shows 100 matching records
and can reveal the rest of the retained history. Clear all also clears health
proof; event tombstones remain only for deduplication and connection status.
