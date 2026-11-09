# 通知渠道配置指南

[English](CHANNELS.md) · **简体中文** · [返回 README](../README.zh-CN.md)

本文按 Nudilo **0.1.0** 的实际界面编写，配置资料核对日期为 **2026-11-05**。平台后台的菜单名称可能随版本、语言和组织权限变化；下文同时给出关键权限名称、字段及官方入口，方便查找。

你不需要写代码、安装 npm 或部署服务器。Nudilo 负责发送通知和私聊绑定；你需要在所选平台创建机器人，把凭据填入桌面客户端。

## 目录

- [先选一种接收方式](#choose)
- [开始前准备与通用操作](#basics)
- [飞书：个人私聊、群聊、常见问题](#feishu)
- [Lark：个人私聊、群聊、常见问题](#lark)
- [钉钉：个人私聊、群聊、常见问题](#dingtalk)
- [Telegram：机器人、私聊、群 ID、频道](#telegram)
- [Slack：应用配置、私聊、频道](#slack)
- [验证自动通知与手机提醒](#verify)
- [修改、重新绑定和删除配置](#manage)
- [自定义通知文案与语言](#content)
- [常见问题与名词说明](#faq)

<a id="choose"></a>

## 先选一种接收方式

**只提醒自己，选择「个人私聊」。希望团队也看到提醒，选择「群聊 / 频道」。** 私聊消息出现在你与自己配置的机器人的对话里，不是 IM 系统助手或 Nudilo 官方账号发来的消息。

| 平台 | 个人私聊需要填写 | 群聊 / 频道需要填写 | 私聊接收账号如何确定 |
| --- | --- | --- | --- |
| 飞书 | App ID、App Secret | 群机器人 Webhook；启用签名时再填签名密钥 | 私聊发送验证码，自动识别 Open ID |
| Lark | App ID、App Secret | 群机器人 Webhook；启用签名时再填签名密钥 | 私聊发送验证码，自动识别 Open ID |
| 钉钉 | Client ID、Client Secret | 群机器人 Webhook；启用加签时再填加签密钥 | 私聊发送验证码，自动识别用户 ID 和 Robot Code |
| Telegram | Bot Token | Bot Token、群聊 / 频道 ID | 打开绑定链接或发送验证码，自动识别私聊 Chat ID |
| Slack | Bot User OAuth Token | Incoming Webhook URL | 私聊发送验证码，自动识别成员 ID |

可以同时配置多个平台，例如同时发到飞书私聊和 Telegram。**同一个平台只有一个当前接收目标**：飞书私聊与飞书群聊不能同时启用，Slack 也不能同时选择多个频道。

<a id="basics"></a>

## 开始前准备与通用操作

### 准备好这些内容

1. 电脑上已安装并打开 Nudilo，Codex 或 Claude Code 已正常使用过。
2. 手机已登录准备接收通知的 IM 账号。
3. 电脑能访问该平台的接口。手机能联网，不代表电脑上的 Nudilo 一定能访问平台。
4. 企业平台可能需要管理员允许创建、安装或发布应用。没有权限时，请管理员协助完成平台侧配置，再把该应用所需凭据填入 Nudilo。
5. 尽量为 Nudilo 单独创建机器人，避免与其他机器人程序争抢绑定消息。

下面的 URL、ID、验证码示例都是占位示例，不能直接作为真实配置使用。App Secret、Token 和 Webhook 都相当于机器人密码，请从自己的平台后台复制，不要把真实值发到公开仓库、群聊或问题截图中。

### 在 Nudilo 中从哪里开始

1. 打开左侧「通知渠道」。
2. 选择飞书、Lark、钉钉、Telegram 或 Slack。
3. 如果已经显示「已配置」的概览，先点击「编辑配置」。
4. 在「发送到」选择「个人私聊」或「群聊 / 频道」。
5. 按对应平台的章节填写。
6. 私聊先「绑定账号」，群聊不需要绑定账号。
7. 点击「保存并测试」，再去手机上查看目标会话。

### 绑定账号的完整过程

1. 先填好应用凭据或 Bot Token，再点击「绑定账号」。
2. 等到页面显示正在等待验证码，并出现 `nudilo-…` 验证码。
3. 点击「复制」，用**希望收到提醒的那个账号**，把完整验证码作为一条新的纯文本消息，私聊发给目标机器人。不要加引号、说明文字或代码块，也不要转发旧消息。
4. 保持 Nudilo 的当前渠道页面不变。可以切换到 IM 软件或浏览器，但不要在 Nudilo 内换页面、换平台或换发送方式。
5. 看到「账号已绑定」后，点击「保存并测试」。只有绑定成功而没有保存，新的接收账号还不会生效。

验证码每次随机生成，有效期为 **3 分钟**。成功、取消、超时或切换 Nudilo 页面后，临时接收会结束。重新开始时必须发送新验证码。机器人不需要在聊天窗口回复验证码；以 Nudilo 中的绑定状态为准。

Telegram 还支持「打开机器人」链接，具体见 [Telegram 私聊](#telegram-dm)。

### 保存后页面应该是什么样

| 操作或状态 | 含义 |
| --- | --- |
| 已配置 | 已保存配置；不代表每一条消息都能成功投递 |
| App Secret / Token「已保存」 | 密钥保存在本地，不会在概览中明文显示 |
| 账号已绑定 | 已识别私聊接收账号；首次配置仍需保存 |
| 保存 | 保存当前配置，不主动发送测试消息 |
| 保存并测试 | 先保存，再向当前目标发送一条测试消息 |
| 发送测试 | 使用已保存的配置发送测试消息 |
| 编辑配置 | 自动填入本地保存的内容，密钥保持遮挡 |
| 取消修改 | 放弃这次编辑，恢复原来的已保存配置 |

如果「保存并测试」中的发送失败，**已经保存成功的配置仍然保留**。到「通知记录」查看错误，修正后再测试即可，不必重新填写所有内容。

<a id="feishu"></a>

## 飞书

飞书与国际版 Lark 是两个独立渠道。使用飞书账号和 `open.feishu.cn` 后台时，请选择 Nudilo 的「飞书」。

<a id="feishu-dm"></a>

### 飞书个人私聊

最终需要：**App ID + App Secret + 一次验证码绑定**。不需要填写邮箱、Open ID、手机号或群 Webhook。

#### 1. 创建应用并添加机器人

1. 打开[飞书开发者后台](https://open.feishu.cn/app)，登录你所在组织的账号。
2. 创建「企业自建应用」，填写便于识别的名称，例如 `Nudilo Notifications`，以及后台要求的描述、图标。
3. 进入应用，在「添加应用能力」中添加「机器人」；已有应用则检查机器人能力已开启。
4. 在「凭证与基础信息」找到 **App ID** 和 **App Secret**。App ID 常见形式为 `cli_…`。
5. 在 Nudilo 选择「飞书 → 个人私聊」，把这两个值填入同名输入框，暂时不用保存。

这里要使用应用机器人。群设置中添加的自定义机器人只能作为群聊 Webhook 使用，不能拿来完成本节的私聊绑定。机器人类型的区别可查阅[飞书自定义机器人指南](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot)。

#### 2. 开通两项应用权限

在应用的「权限管理」中，按权限标识搜索并申请：

| 权限标识 | 用途 |
| --- | --- |
| `im:message:send_as_bot` | 以机器人身份向你发送通知 |
| `im:message.p2p_msg:readonly` | 绑定时接收你私聊发给机器人的验证码 |

选择**应用身份**的权限。Nudilo 的自动绑定不需要通讯录权限，也不需要读取所有群消息。权限与消息事件的关系见[消息概述](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1)。

如果后台提供「批量导入权限」，可使用下面这份只包含所需权限的 JSON；没有该入口则逐项添加：

```json
{
  "scopes": {
    "tenant": [
      "im:message:send_as_bot",
      "im:message.p2p_msg:readonly"
    ],
    "user": []
  }
}
```

申请后留意是否仍处于待审核或待发布状态。

#### 3. 配置长连接和接收消息事件

1. 在后台打开「事件与回调 → 事件配置」。
2. 回到 Nudilo，点击「绑定账号」，等待连接就绪。
3. 在后台编辑订阅方式，选择「使用长连接接收事件」，然后保存。
4. 添加「接收消息 v2.0」事件，事件标识为 **`im.message.receive_v1`**。
5. 保存事件配置。

**为什么要先点击绑定？** 飞书保存长连接设置时需要检测到在线连接。Nudilo 只在绑定期间建立接收连接，因此应在等待绑定的 3 分钟内保存后台设置；超时就重新点击绑定。不要为这一步另外搭建服务器或填写回调 URL。这个顺序对应飞书的[长连接配置要求](https://open.feishu.cn/document/ukTMukTMukTM/uYDNxYjL2QTM24iN0EjN/event-subscription-configure-)。

这里配置的是「事件配置」中的消息接收事件；Nudilo 不需要卡片按钮回调、Verification Token 或 Encrypt Key。

#### 4. 发布并确认自己可以使用

1. 进入「版本管理与发布」，创建一个版本，填写版本号和更新说明。
2. 设置应用可用范围，确保包含准备接收通知的账号。
3. 提交发布；如果组织要求审批，等待管理员通过。
4. 之后修改权限、机器人能力或事件配置时，按后台提示再次发布。
5. 在飞书客户端搜索机器人名称并打开单聊。找不到时，先检查当前组织、可用范围和发布状态。

应用可用范围会影响能否发送私聊，具体限制见[飞书发送消息 API](https://open.feishu.cn/document/server-docs/im-v1/message/create)。

#### 5. 正式绑定并测试

1. 回到 Nudilo，之前的绑定如果已结束，重新点击「绑定账号」。
2. 复制本次验证码，私聊发给刚才创建的机器人。
3. 等到 Nudilo 显示已绑定账号。
4. 点击「保存并测试」。
5. 手机上打开与该机器人的私聊，应该能看到含 `Nudilo`、`Codex` 的连接测试消息。

### 飞书群聊

1. 在飞书客户端打开要接收提醒的群。
2. 从群设置进入「群机器人 / 机器人」，添加「自定义机器人」。如果没有添加权限，请群主或管理员协助。
3. 填写机器人名称并完成创建，复制生成的完整 Webhook。
4. 在 Nudilo 选择「飞书 → 群聊 / 频道」，把 Webhook 填入「Webhook URL」。
5. 按下面的安全设置填写签名密钥，点击「保存并测试」。
6. 在这个群里确认测试消息。群 Webhook 固定对应它所在的群。

飞书群机器人的 URL 应类似：

```text
https://open.feishu.cn/open-apis/bot/v2/hook/<YOUR_WEBHOOK_ID>
```

| 飞书后台的安全设置 | Nudilo 中的操作 |
| --- | --- |
| 签名校验 | 展开「签名（可选）」，粘贴机器人签名密钥 |
| 自定义关键词 | 同时允许 `Codex` 和 `Claude Code`；正式通知包含实际助手名称，测试通知包含两者 |
| IP 白名单 | 由管理员允许这台电脑实际对外请求使用的公网出口 IP；换网络或代理后要重新核对 |

启用了哪些校验，就要满足哪些条件。后台启用了签名，Nudilo 的签名密钥就不是可留空项；这里的签名密钥也不是私聊应用的 App Secret。安全规则见[飞书自定义机器人指南](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot)。

### 飞书常见问题

| 现象 | 排查顺序 |
| --- | --- |
| 后台提示没有长连接 | 在 Nudilo 发起绑定并等到连接就绪；保持等待，再保存后台设置 |
| 能找到机器人但没有输入框 | 检查机器人能力、私聊接收权限、消息事件及发布状态；重新打开会话 |
| 发了验证码但一直等待 | 检查发给的是应用机器人、是私聊、验证码是本次新生成的；确认没有其他程序连接同一个应用 |
| 绑定成功但发送失败 | 接收权限不等于发送权限；检查 `im:message:send_as_bot`、发布审批和账号可用范围 |
| 群消息关键词校验失败 | 检查后台同时允许 `Codex` 和 `Claude Code`，注意拼写和大小写 |
| 群消息签名失败 | 重新复制签名密钥，确认不是 App Secret，并检查电脑时间是否正确 |
| 提示 Webhook 格式不正确 | 使用完整的飞书 v2 群机器人地址，不要填 Lark 地址、应用 ID 或后台页面地址 |

<a id="lark"></a>

## Lark

Lark 使用 [Lark Developer Console](https://open.larksuite.com/app)，接口域名为 `open.larksuite.com`。不要把飞书的凭据或 Webhook 填到 Lark 渠道。

### Lark 个人私聊

1. 登录 Lark 开发者后台，在目标组织创建 **Custom App**。
2. 在 **Add App Capability / Features** 添加 **Bot**。
3. 在 **Credentials & Basic Info** 复制 App ID 和 App Secret，填入 Nudilo 的「Lark → 个人私聊」。
4. 在 **Permissions & Scopes** 添加应用身份权限 `im:message:send_as_bot` 和 `im:message.p2p_msg:readonly`。二者分别用于发送通知和接收绑定验证码。
5. 打开后台 **Events & Callbacks → Event Configuration**，然后在 Nudilo 点击「绑定账号」。
6. 等待 Nudilo 连接就绪，在后台选择通过 **long connection** 接收事件并保存。
7. 添加 **Receive message** 事件 **`im.message.receive_v1`**，保存配置。
8. 在 **Version Management & Release** 创建并发布版本，将接收账号纳入 **Availability**，完成组织要求的审批。
9. 在 Lark 客户端搜索此应用的机器人，打开它的私聊。
10. 回到 Nudilo 发起一次新的绑定，将新验证码发到该私聊；识别成功后点击「保存并测试」。

Lark 的[应用创建与机器人能力说明](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/mcp_integration/develop-mcp-intelligent-assistant-bot)可用于查找后台入口；该官方教程还包含其他产品能力，Nudilo 只需要上面列出的两项权限和消息事件，不需要照搬教程中的全部权限或部署步骤。

第一次保存长连接时，如果提示没有连接，保持 Nudilo 的绑定等待状态再保存；超时后重新绑定即可。配置界面中的具体英文名称以当前后台为准。Nudilo 不需要邮箱、Open ID、OAuth 回调地址或额外的本地脚本。

发布后如果找不到应用，检查当前 Lark 组织和账号是否在可用范围。可用范围与机器人输入框的问题可参考 [Lark 消息常见问题](https://open.larksuite.com/document/server-docs/im-v1/faq)。

### Lark 群聊

1. 在目标群打开 **Settings → Bots → Add Bot → Custom Bot**。
2. 设置机器人名称，完成创建并复制 Webhook。
3. 在 Nudilo 选择「Lark → 群聊 / 频道」，填入「Webhook URL」。
4. 如果启用了 **Signature verification**，展开「签名（可选）」填写对应密钥。
5. 如果启用了关键词，同时允许 `Codex` 和 `Claude Code`；如启用了 IP 白名单，允许电脑当前的公网出口 IP。
6. 点击「保存并测试」，在目标群检查消息。

群机器人创建入口见 [Lark 帮助中心](https://www.larksuite.com/hc/en-US/articles/897957711856-http-requests-sending-group-messages-applicable-for-external-groups)，安全设置见 [Lark 自定义机器人指南](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/bot-v3/use-custom-bots-in-a-group)。

Lark 的 Webhook 格式应类似：

```text
https://open.larksuite.com/open-apis/bot/v2/hook/<YOUR_WEBHOOK_ID>
```

### Lark 常见问题

| 现象 | 检查内容 |
| --- | --- |
| 凭据正确却连接失败 | 凭据必须来自 Lark，Nudilo 也必须选择 Lark；确认电脑能访问 Lark 接口 |
| 不能保存事件订阅方式 | 先在 Nudilo 建立绑定连接，再保存 long connection 设置 |
| 绑定没有响应 | 私聊接收权限、`im.message.receive_v1`、发布版本、当前验证码，以及是否有其他接收程序 |
| 能绑定不能发送 | 发送权限、组织审批、可用范围，是否更换过应用凭据 |
| 群消息失败 | Webhook 域名、签名密钥、关键词及 IP 白名单逐项核对 |

<a id="dingtalk"></a>

## 钉钉

### 钉钉个人私聊

需要：**Client ID（AppKey）+ Client Secret（AppSecret）**。Nudilo 从绑定消息自动取得用户 ID 和 Robot Code，不需要你额外查找。

#### 1. 创建企业内部应用

1. 登录[钉钉开发者后台](https://open-dev.dingtalk.com/)，选择接收账号所在的组织。
2. 在应用开发区域创建「企业内部应用 / 钉钉应用」。使用新版以应用为中心的入口，再为应用添加机器人能力。
3. 填写应用名称、描述等必填项。
4. 在「凭证与基础信息」复制 **Client ID** 和 **Client Secret**。旧界面可能标为 **AppKey**、**AppSecret**，它们分别对应 Nudilo 的两个输入框。
5. 在 Nudilo 选择「钉钉 → 个人私聊」，填入这两个值。

官方入口及新旧名称对照见[创建机器人应用](https://open-dingtalk.github.io/developerpedia/docs/explore/tutorials/stream/bot/java/create-bot/)。

#### 2. 开启机器人和权限

1. 在应用内「添加应用能力」或「机器人与消息推送」中开启机器人，设置名称和后台要求的资料。
2. 将机器人的**消息接收模式设置为 Stream 模式**，保存机器人配置。
3. 在「权限管理」中选择机器人相关权限，开通发送单聊消息所需权限。可搜索「企业内机器人发送消息」；后台可能显示权限标识 `qyapi_robot_sendmsg`。
4. 创建并发布应用版本，把接收账号加入可见 / 可用范围，完成必要审批。

权限申请入口见[钉钉权限申请](https://open.dingtalk.com/document/orgapp/apply-for-permissions-8)，`qyapi_robot_sendmsg` 的后台配置可参照[阿里云官方钉钉接入说明](https://help.aliyun.com/zh/starops/user-guide/connect-to-dingtalk)。该说明中的卡片权限用于其他产品，Nudilo 的文本通知不需要一并添加。

#### 3. 绑定并测试

1. 在钉钉客户端搜索机器人名称；如果搜索结果分类较多，可查看「功能」分类，然后打开单聊。
2. 在 Nudilo 点击「绑定账号」，等待显示验证码。
3. 用同一组织内的接收账号，私聊发送该验证码。不要在调试群里 `@机器人` 代替私聊绑定。
4. 等待 Nudilo 显示已绑定，点击「保存并测试」。
5. 在手机上的机器人单聊中确认测试消息。

查找机器人方法见[钉钉官方单聊体验说明](https://open-dingtalk.github.io/developerpedia/docs/explore/tutorials/stream/bot/python/test-bot/)。Nudilo 当前绑定要求发送者与机器人属于同一组织；它使用 [BatchSendOTO](https://open-dev.dingtalk.com/apiExplorer#/?devType=org&api=robot_1.0%23BatchSendOTO) 向识别出的一个用户发送通知。

Stream 由 Nudilo 在绑定期间连接，不需要填写公网消息接收地址，也不需要安装钉钉 SDK。相关机制见[钉钉官方 Stream SDK](https://github.com/open-dingtalk/dingtalk-stream-sdk-go)。

### 钉钉群聊

当前 Nudilo 的钉钉群聊方式使用**自定义群机器人 Webhook**，不使用企业应用的群消息 API。

1. 打开目标群，进入「群设置 → 机器人 → 添加机器人」；部分客户端将入口称为「智能群助手」。
2. 选择「自定义机器人」，填写名称并按后台要求设置安全选项。已有群机器人可直接进入其设置页面。
3. 复制完整 Webhook，保留 `access_token=` 后面的全部内容。
4. 在 Nudilo 选择「钉钉 → 群聊 / 频道」，填入「Webhook URL」。
5. 如启用「加签」，展开 Nudilo 的「签名（可选）」，填写机器人提供的 `SEC…` 密钥。
6. 如使用关键词校验，同时允许 `Codex` 和 `Claude Code`；如使用 IP 白名单，核对电脑的公网出口 IP。
7. 点击「保存并测试」，到这个群查看消息。

官方步骤见[创建自定义机器人](https://open.dingtalk.com/document/orgapp/custom-bot-creation-and-installation)和[自定义机器人安全设置](https://open.dingtalk.com/document/orgapp/customize-robot-security-settings)。发送通知不需要开启 Outgoing 机制。

```text
https://oapi.dingtalk.com/robot/send?access_token=<YOUR_ACCESS_TOKEN>
```

这里不要填写 AppKey、Robot Code、临时 `sessionWebhook` 或调试接口地址。签名参数由 Nudilo 生成，不需要手动给 URL 拼接 `timestamp`、`sign`。

如果你的群或组织没有自定义机器人入口，先让管理员确认该群是否支持。不要把企业应用凭据填进 Webhook 输入框；当前版本尚未提供企业应用群消息配置，可改用钉钉个人私聊，或选择其他可用群聊渠道。

### 钉钉常见问题

| 现象 | 排查顺序 |
| --- | --- |
| 搜不到机器人 | 当前组织、应用发布状态、可见范围、机器人能力是否启用 |
| Stream 连接失败 | Client ID / Secret 是否来自同一个应用；消息接收模式是否保存为 Stream |
| 群里发了验证码但绑定失败 | 个人私聊绑定只接受单聊，请打开机器人的私聊重新发送新验证码 |
| 无法识别用户或 Robot Code | 使用同一组织的账号；确认应用已发布，重新发起绑定 |
| 绑定成功但单聊发送失败 | 检查机器人发送权限和可用范围；仅收到消息不代表有主动发送权限 |
| 群消息被拒绝 | 核对 `access_token`、加签密钥、`Codex` / `Claude Code` 关键词、IP 白名单及电脑时间 |

<a id="telegram"></a>

## Telegram

Telegram 私聊、群聊和频道都使用同一种 **Bot Token**。区别在接收目标：私聊自动绑定，群聊 / 频道需要填写地址。

### 创建自己的机器人

1. 在 Telegram 打开官方 [@BotFather](https://t.me/BotFather)。
2. 发送 `/newbot`，按提示填写显示名称。
3. 再设置一个未被占用、以 `bot` 结尾的机器人用户名，例如 `your_name_nudilo_bot`。
4. 创建成功后复制 BotFather 给出的 Token，必须包含数字部分、冒号和后面的完整字符串。
5. 在 Nudilo 选择「Telegram」，把 Token 填入「Bot Token」。
6. 记住机器人的 `@用户名`，后面打开私聊或加群会用到；它不是 Token。

机器人创建与 Token 说明见 [Telegram 官方教程](https://core.telegram.org/bots/tutorial#obtain-your-bot-token)和 [BotFather 说明](https://core.telegram.org/bots/features#botfather)。Nudilo 已包含机器人发送功能，不需要继续跟随官方教程编写机器人程序。

<a id="telegram-dm"></a>

### Telegram 个人私聊

1. 选择「个人私聊」，确认 Bot Token 已填写。
2. 点击「绑定账号」，等待出现验证码和「打开机器人」。
3. 点击「打开机器人」，在 Telegram 中按 **Start / 开始**。链接会携带本次验证码。
4. 如果没有出现 Start，或者打开后没有绑定成功，复制 Nudilo 的验证码，作为一条新消息直接发给自己的机器人。
5. Nudilo 显示已绑定后，点击「保存并测试」。

一定要发送给**你创建的机器人**，不是 BotFather。普通 `/start` 可以开启聊天，但没有本次验证码时不足以完成 Nudilo 绑定。私聊不需要查询 Chat ID，也不能把个人 `@用户名` 填入群聊 / 频道 ID。

### Telegram 群聊

1. 把机器人添加到目标群。若无法邀请，检查 BotFather 中该机器人是否允许加入群组。
2. 确认机器人有发送文本消息的权限；群聊通常无需给它全部管理员权限。
3. 按下一节获取群的 Chat ID。
4. 在 Nudilo 选择「群聊 / 频道」。
5. 填写 Bot Token 和「群聊 / 频道 ID」，后者使用完整负数，例如 `-1001234567890`。
6. 点击「保存并测试」，在群中确认消息。

#### 不使用第三方查 ID 机器人的获取方法

这个方法直接读取 Telegram 官方 Bot API，适合刚创建的专用机器人：

1. 如果 Nudilo 正在绑定，先点击取消；也不要同时运行其他读取这个机器人消息的程序。
2. 在目标群发送一条明确指向机器人的命令，例如 `/nudilo_setup@your_name_nudilo_bot`。用户名要换成你的机器人用户名，不带第二个 `@`。
3. 在浏览器隐私窗口的**地址栏**打开下面的地址，把 `<YOUR_BOT_TOKEN>` 整段替换为自己的 Token。不要把含 Token 的地址输入搜索框或分享出去。

   ```text
   https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates
   ```

4. 在返回内容中找到刚才那条消息，确认 `chat.title` 是目标群，读取 **`message.chat.id`**。

   ```json
   {
     "message": {
       "chat": {
         "id": -1001234567890,
         "title": "My Nudilo alerts",
         "type": "supergroup"
       },
       "text": "/nudilo_setup@your_name_nudilo_bot"
     }
   }
   ```

5. 复制整个负数到 Nudilo，包括开头的 `-`。不要复制 `from.id`、`message_id` 或 `update_id`。
6. 如果 `result` 为空，再发一条新命令并刷新；如果是 Webhook 冲突，使用专用机器人，不要删除其他服务的 Webhook。
7. 完成后关闭隐私窗口。此操作不需要 `offset`、`deleteWebhook` 或 `drop_pending_updates` 参数。

群隐私模式允许机器人接收明确发给它的命令，因此通常无需为了查 ID 关闭隐私模式。接收规则见 [Telegram Bot FAQ](https://core.telegram.org/bots/faq#what-messages-will-my-bot-get)，返回字段见 [getUpdates](https://core.telegram.org/bots/api#getupdates)。

### Telegram 频道

**公开频道：**

1. 将机器人加入频道的管理员列表，并允许它发布消息。
2. 在 Nudilo 选择「群聊 / 频道」，填写 Bot Token。
3. 「群聊 / 频道 ID」填写频道的公开用户名，例如 `@your_nudilo_alerts`；不是显示名称，也不是 `https://t.me/…` 链接。
4. 保存并测试，确认消息出现在频道中。

**私有频道：**

1. 同样添加机器人并授予发布消息权限。
2. 自己在频道发一条新的普通消息。
3. 用上面的官方 `getUpdates` 方法找到对应的 **`channel_post.chat.id`**，确认频道名称后复制完整负数。
4. 把负数 ID 填入 Nudilo，保存并测试。不需要为了取得 ID 把私有频道改成公开。

当前 Nudilo 不提供 Telegram 话题 ID 或 `message_thread_id` 输入项，不能指定某个论坛群话题。发送目标的 API 规则见 [sendMessage](https://core.telegram.org/bots/api#sendmessage)。

### Telegram 常见问题

| 现象 | 处理方式 |
| --- | --- |
| Token 格式错误 | 从 BotFather 重新复制完整 Token，不能填 `@用户名` 或机器人链接 |
| 点击 Start 后仍未绑定 | 使用 Nudilo 本次「打开机器人」链接，或直接发送本次验证码；普通 `/start` 不带验证码 |
| 提示 Webhook 或其他更新读取器冲突 | 使用专用机器人；Nudilo 不会替你删除既有 Webhook |
| 提示待处理消息过多 | 当前绑定不会消费无关更新；专用机器人能避免旧更新积压阻挡验证码 |
| 群 / 频道找不到或发送被拒绝 | 检查完整 Chat ID、机器人是否在目标会话、是否有发送权限；频道需发布权限 |
| 群升级后旧 ID 不再工作 | 重新取得当前群 ID 并保存 |
| 手机 Telegram 正常，Nudilo 超时 | 检查电脑访问 `api.telegram.org` 的网络与代理；手机连接不代表桌面接口可达 |

<a id="slack"></a>

## Slack

Slack 私聊使用机器人 OAuth Token，频道使用 Incoming Webhook。两种字段不能互换。

### Slack 个人私聊：新建应用

#### 1. 使用 Nudilo 提供的应用配置

1. 在 Nudilo 打开「Slack → 个人私聊」。已配置时先点击「编辑配置」。
2. 展开「配置帮助」，点击「复制应用配置」。也可以打开[仓库内同一份 JSON](../internal/desktop/slack-manifest.json)并复制全部内容。
3. 打开 [Slack 应用管理](https://api.slack.com/apps)，选择 **Create New App → From a manifest**。
4. 选择要接收通知的工作区，将 JSON 粘贴到 manifest 编辑器；如有 JSON / YAML 选项，选择 JSON。
5. 按页面提示预览配置并创建应用。

这份配置预设三项机器人权限，打开 Messages 页面并允许用户发送消息，也关闭了 Nudilo 当前不使用的 Socket Mode 和 Token Rotation。创建过程可参考 [Slack App Manifest 官方指南](https://docs.slack.dev/app-manifests/configuring-apps-with-app-manifests/)。

#### 2. 安装应用并取得 Token

1. 在 Slack 应用后台打开 **OAuth & Permissions**。
2. 找到 **Install to Workspace**，按页面提示安装和授权；如果需要审批，请工作区管理员批准。
3. 安装后复制 **Bot User OAuth Token**，它以 **`xoxb-`** 开头。
4. 粘贴到 Nudilo 的同名输入框。

不要复制 **Client Secret**、**Signing Secret**、`xapp-` 应用级 Token 或 `xoxp-` 用户 Token。Nudilo 只接受本模式的机器人 Token；不需要 OAuth Redirect URL、Socket Mode 或事件回调服务器。

#### 3. 找到应用并绑定

1. 在正确的 Slack 工作区中，从 **Apps / 应用** 搜索刚创建的应用。
2. 打开应用的 **Messages / 消息** 页面，确认可以输入消息。
3. 在 Nudilo 点击「绑定账号」，等待显示本次验证码。
4. 用准备接收通知的成员账号，把验证码作为一条新的纯文本消息发给这个应用。不要发给 Slackbot、频道或自己与他人的私聊，也不要编辑旧消息来替换验证码。
5. 等待 Nudilo 显示已绑定，点击「保存并测试」。
6. 在手机 Slack 中打开与该应用的对话，检查测试消息。

Nudilo 绑定期间用 [conversations.list](https://docs.slack.dev/reference/methods/conversations.list/) 查找机器人的私聊，用 [conversations.history](https://docs.slack.dev/reference/methods/conversations.history/) 检查本次验证码，再通过 [chat.postMessage](https://docs.slack.dev/reference/methods/chat.postMessage/) 发送通知。这是当前实现不需要 Socket Mode 和 Event Subscriptions 的原因。

### Slack 个人私聊：使用已有应用

在 **OAuth & Permissions → Scopes → Bot Token Scopes** 检查以下权限：

| Bot Scope | Nudilo 用途 |
| --- | --- |
| `chat:write` | 以机器人身份发送通知 |
| `im:read` | 绑定时找到机器人参与的私聊 |
| `im:history` | 绑定时读取新验证码 |

然后完成以下操作：

1. 在 **App Home** 启用 **Messages Tab**。
2. 允许用户向应用发送消息；部分后台文案会同时提到 Slash Commands，不需要额外创建 Slash Command。
3. 修改权限后，回到 **OAuth & Permissions** 重新安装应用，让工作区授权生效。
4. 将当前 Bot User OAuth Token 填入 Nudilo，并重新绑定、保存测试。

Messages 页面设置见 [Slack App Home](https://docs.slack.dev/surfaces/app-home/)。不要为 Nudilo 额外开放全部公开或私有频道的历史权限。当前版本不提供 Slack OAuth Token 自动轮换流程；使用随附 manifest 的配置即可。

### Slack 频道

1. 在 [Slack 应用管理](https://api.slack.com/apps)创建应用，选择 **From scratch**，填写名称并选择工作区；也可使用已有应用。
2. 进入 **Incoming Webhooks**，开启 **Activate Incoming Webhooks**。
3. 点击 **Add New Webhook to Workspace**，选择目标频道并授权。
4. 私有频道需要你先是该频道成员，再进行选择和授权。
5. 复制新生成的 Webhook URL。
6. 在 Nudilo 选择「Slack → 群聊 / 频道」，将 URL 填入「Webhook URL」，点击「保存并测试」。

创建和频道授权步骤见 [Slack Incoming Webhooks 官方指南](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/)。

```text
https://hooks.slack.com/services/<WORKSPACE>/<WEBHOOK>/<SECRET>
```

Webhook 的目标频道在 Slack 授权时确定。要换频道，生成对应频道的新 Webhook，再到 Nudilo 编辑并保存。此模式不需要填写 Bot Token、频道 ID 或 Signing Secret，也不需要先绑定个人账号。

当前版本支持普通 Slack 工作区私聊；Slack Gov 私聊未接入。群聊 Webhook 校验接受 `hooks.slack.com` 和 `hooks.slack-gov.com`。

### Slack 常见问题

| 现象 | 排查顺序 |
| --- | --- |
| 找不到 Bot User OAuth Token | 先把应用安装到正确工作区，再查看 OAuth & Permissions |
| 应用 Messages 页面不能输入 | 启用 Messages Tab，并允许用户发消息；重新打开 Slack 应用对话 |
| `missing_scope` 或权限不足 | 核对三项 Bot Scopes，重新安装应用后再绑定 |
| Token 无效、过期或被撤销 | 从后台取得有效机器人 Token；如果更换 Token，在 Nudilo 重新绑定并保存 |
| 验证码一直未识别 | 检查工作区、目标应用、当前验证码和是否新发送的私聊消息；不要用编辑或转发的旧消息 |
| 应用私聊很多或被限流 | 等待后重试；为个人通知使用单独应用，减少绑定扫描量 |
| 频道测试失败 | 检查 Webhook 是否属于目标频道、频道是否已归档，以及应用是否仍被允许使用 |
| 想填 `#频道名` | 当前群聊模式只需要频道 Webhook，不接受频道名代替 URL |

<a id="verify"></a>

## 验证自动通知与手机提醒

### 第一层：平台配置能否发消息

1. 在配置页点击「保存并测试」，或在已配置概览点击「发送测试」。
2. 打开「通知记录」，查看测试是否投递到平台；失败时先处理显示的错误。
3. 在目标私聊、群或频道里找到测试消息，确认不是发到了另一个组织、账号或频道。

测试成功只表示平台接受了消息，并不等于手机一定发出了声音、横幅或锁屏提醒。

### 第二层：手机是否提醒

1. 检查手机系统是否允许该 IM 显示通知、声音和锁屏提醒。
2. 检查目标会话是否免打扰；频道是否仅提醒被提及的消息。
3. 检查 IM 的通知暂停、工作时间、桌面在线时是否仍通知手机等选项；具体名称因平台而异。
4. 锁屏或切到其他应用后再发一条测试，确认手机实际展示了提醒。

Nudilo 不会用 `@所有人` 强制提醒，群聊只提醒被提及消息时尤其要注意。先确认消息已经在会话中，再排查手机显示设置。

例如飞书可以在电脑或 iPad 在线时关闭手机通知，Slack 也有单独的手机通知选项。操作位置可查阅[飞书通知设置](https://www.feishu.cn/hc/zh-CN/articles/111093338362-在移动端设置消息提示音)和 [Slack 通知设置](https://slack.com/intl/en-gb/help/articles/201355156-Configure-your-Slack-notifications)。

### 第三层：AI 助手的真实事件是否触发

1. 在 Nudilo「概览」确认提醒已开启，所需的完成、回答、授权提醒类型已启用。
2. 刷新对应 AI 助手的关联状态；若提示需要重新关联，按提示处理。
3. 让 Codex 或 Claude Code 完成一个新的简短任务，观察通知记录是否出现新的「本轮已结束」事件。
4. 确认手机接收的是这次真实事件，而不只是之前的连接测试。

已打开的 Codex 会话可以通过本地完成事件监听发送结束提醒；如果是在安装关联前就打开的会话，需要重新打开以加载回答和授权事件。已有 Claude Code 会话关联后未生效时也请重新打开；版本要求与事件覆盖范围见[Claude Code 关联说明](BEHAVIOR.md#claude-code-zh)。

「本轮已结束」指 AI 助手这一轮输出结束，不保证整个任务成功。Nudilo 当前只负责提醒；需要回答、补充资料或授权时，请回到电脑上的对应 AI 助手。

### 根据现象定位问题

| 现象 | 优先检查 |
| --- | --- |
| 绑定阶段失败 | 应用凭据、接收权限、接收模式、当前验证码 |
| 绑定成功，测试失败 | 发送权限、发布状态、接收范围、网络 |
| 测试成功，会话没看到 | 是否看错账号、组织、机器人、群或频道 |
| 会话有消息，手机无横幅 | 手机系统通知、会话免打扰、IM 推送设置 |
| 测试正常，真实任务无记录 | 提醒开关、提醒类型、对应助手的关联及会话是否加载事件 |
| 真实事件有记录但失败 | 记录里的平台错误及对应渠道配置 |
| 记录显示跳过 | 事件发生时是否暂停、关闭该类型或没有配置渠道 |
| 退出 Nudilo 后收不到 | Nudilo 需要保持运行；macOS 关闭窗口后仍在菜单栏运行，选择「退出」则停止 |

<a id="manage"></a>

## 修改、重新绑定和删除配置

### 修改已有配置

1. 打开对应平台，点击「编辑配置」。
2. 已保存内容会自动填入，密钥显示为遮挡状态，不是空值。
3. 修改所需内容；换 App ID、App Secret 或 Bot Token 后，私聊需重新绑定。
4. 点击「保存并测试」。不想保留修改时，点击「取消修改」。

### 换一个私聊接收账号

1. 进入编辑配置，点击「重新绑定」。
2. 用新接收账号向同一个机器人发送本次验证码。
3. 新账号识别成功后保存。它会替换原接收账号，不会新增第二个私聊接收人。
4. 绑定失败或取消时，已保存的旧目标不会被修改。

### 在私聊与群聊之间切换

编辑时切换发送方式，填写或确认所需内容，再保存。切换前的另一种方式配置仍保留在本地；但每个平台只有保存后的方式处于启用状态。Telegram 两种方式共用一个 Bot Token，更换 Token 后要检查两边目标与新机器人的关系。

更换接收目标后，旧目标下等待重试的通知不会转发到新目标，后续新事件才使用新配置。

### 移除渠道或重置凭据

点击「移除渠道」并确认后，会删除 Nudilo 中该平台的本地配置并停用该渠道，不会在平台后台删除机器人、卸载应用或撤销 Token。如果要彻底停用机器人，请另外去对应平台处理。

如果平台侧重置了 Secret、Token 或 Webhook，到 Nudilo 更新相应字段；私聊更新应用凭据后重新绑定。没有「载入钥匙串」或旧凭据导入步骤。

<a id="content"></a>

## 自定义通知文案与语言

自定义是可选功能，不修改时继续使用默认文案。飞书、Lark、钉钉、Telegram 和 Slack 共用每种语言的同一套文案，私聊和群聊／频道均适用。两种助手共用已保存的渠道，个人私聊无需重新绑定；群机器人启用关键词校验时，请同时允许 `Codex` 和 `Claude Code`。

1. 打开「设置 → 通知文案 → 自定义」，此处显示的是软件当前使用的语言。
2. 修改「通知标题」（单行，最多 80 个字符）。四类提醒共用标题，发送时始终附带实际来源（`Codex` 或 `Claude Code`）和提醒类型。
3. 选择「本轮已结束」「需要回答」「需要授权」或「异常中断」，修改对应的「消息正文」（每类最多 1,000 个字符）。
4. 将光标放在正文中，点击「AI 助手」「项目」「时间」「摘要」插入变量；选中文字时会替换所选内容。预览随输入更新，可以切换 Codex 和 Claude Code；使用示例数据，不会实际发送消息。
5. 点击「保存」，应用当前语言的标题和四类正文。「取消」放弃当前语言尚未保存的修改；「恢复默认」先重置当前语言的草稿，再点击「保存」才生效。

| 变量 | 发送时替换为 |
| --- | --- |
| `{agent}` | 实际来源：`Codex` 或 `Claude Code` |
| `{project}` | 项目目录名，不含完整路径 |
| `{time}` | 按通知语言和电脑时区显示的事件时间 |
| `{summary}` | 可用的助手摘要，最多 500 个字符，需主动开启 |

变量用于正文，其他花括号表达式会在输入框下方提示错误。标题或正文留空时使用当前语言的默认内容。变量展开后过长的消息会自动截短；示例达到长度限制时，预览会提示。实际长度取决于项目名和摘要。

**摘要默认关闭。** 先开启「设置 → 通用 → 附带内容摘要」才能使用摘要插入按钮。关闭后，即使已有正文中包含 `{summary}`，也不会发送摘要。部分事件本身没有摘要，秘密输入问题也不会提供摘要。删除变量可让对应文案省略摘要；本地是否采集摘要仍由通用设置中的开关控制。Claude Code 的 MCP 输入请求和 API 错误不提供摘要。已有自定义文字会原样保留；若曾写死助手名称，可以自行换成 `{agent}`。

### 中英文如何选择

- 在「设置 → 外观 → 语言」选择「简体中文」，提醒类型、默认文案和日期即使用中文；选择「English」则使用英文。
- 「跟随系统」采用软件匹配到的系统语言；不支持的系统语言回退为英文。
- 中英文自定义文案分别保存，切换软件语言后即可编辑另一份。尚未自定义的语言使用自己的默认文案，不会套用另一种语言的自定义内容。
- 项目名、摘要和你自行输入的文字保留原文，不自动翻译。在软件内切换语言不会丢失另一种语言尚未保存的草稿。
- 新发送和重试均采用每次开始发送时的语言、已保存文案和摘要设置；已经发送的消息不会改变。
- 「发送测试」采用当前标题和语言，正文固定为连接测试说明。四类事件文案可在编辑器中预览，或保存后通过新的真实 Codex 或 Claude Code 事件确认。

<a id="faq"></a>

## 常见问题与名词说明

### 配置保存在什么地方？

应用密钥、Token、Webhook 和接收账号保存在本机加密凭据文件中，不使用 macOS 钥匙串或其他系统凭据库。Nudilo 读取的是本地保存值，因此重新打开配置时无需再次载入凭据。文件位置与备份方式见[本地数据说明](BEHAVIOR.md#local-data)。

### 为什么已经能发送，还需要接收消息权限？

接收权限用于一次性的验证码绑定，以便自动识别你是谁。绑定完成后临时接收连接关闭；Nudilo 不把日常私聊当作 AI 助手指令，不保存绑定时收到的聊天内容。之后需要重新绑定时，还会用到接收权限。

### 我能在手机上回复 Codex 或 Claude Code 吗？

当前不能。手机上的消息是提醒，回复机器人不会传回任一 AI 助手，也不会自动批准操作。

### 关掉窗口、睡眠、退出有什么区别？

- 关闭主窗口：有系统托盘时继续运行；macOS 同时隐藏 Dock 图标，菜单栏图标保留。
- 电脑睡眠或断网：无法立即投递，恢复运行与联网后按通知队列状态处理。
- 退出 Nudilo：通知服务停止。想在登录后自动运行，可在设置中开启登录启动。

### 表单名词对照

| 名词 | 是什么 | 从哪里获取 |
| --- | --- | --- |
| App ID / Client ID / AppKey | 应用的标识 | 企业平台应用凭证页 |
| App Secret / Client Secret / AppSecret | 配合应用 ID 使用的密钥 | 同一个应用的凭证页 |
| Bot Token | 操作机器人所用的凭据 | Telegram BotFather，或 Slack OAuth 页面 |
| Webhook URL | 把通知投递到固定群或频道的完整地址 | 群机器人设置，或 Slack Incoming Webhooks |
| 签名密钥 | 群机器人校验请求来源的密钥 | 飞书、Lark、钉钉群机器人的安全设置 |
| Open ID / 用户 ID / 成员 ID | 平台识别私聊接收人的内部标识 | Nudilo 自动绑定，不需要手填 |
| Chat ID | Telegram 的会话标识 | 私聊自动绑定；群和私有频道按本文步骤查询 |
| Stream / 长连接 | Nudilo 临时接收绑定消息的连接方式 | 在平台后台启用，由 Nudilo 建立连接 |
| 可用 / 可见范围 | 哪些组织成员可以使用应用 | 企业平台的应用发布配置 |

如果仍然失败，记录平台、发送方式、失败发生在绑定还是发送阶段，以及「通知记录」中的错误文字即可。提交问题时遮住密钥、Token、Webhook 和个人接收标识。

日常的连接诊断、定时暂停、项目分流和通知筛选见[使用指南](USAGE.zh-CN.md)。
