# Notification channel setup guide

**English** · [简体中文](CHANNELS.zh-CN.md) · [Back to README](../README.md)

This guide follows the actual interface in **Nudilo 0.1.0**. Platform setup references were reviewed on **2026-11-05**. Console labels can vary with language, organization permissions and platform updates; permission identifiers, field mappings and official links are included to help you find the right settings.

You do not need to write code, install npm or deploy a server. Nudilo handles notification delivery and account pairing. Your part is to create a bot on your chosen platform and enter its credentials in the desktop app.

## Contents

- [Choose a destination](#choose)
- [Preparation and common steps](#basics)
- [Feishu: personal DMs, groups and troubleshooting](#feishu)
- [Lark: personal DMs, groups and troubleshooting](#lark)
- [DingTalk: personal DMs, groups and troubleshooting](#dingtalk)
- [Telegram: bot creation, DMs, group IDs and channels](#telegram)
- [Slack: app setup, DMs and channels](#slack)
- [Verify automatic notifications and phone alerts](#verify)
- [Edit, pair again or remove a destination](#manage)
- [Customize notification text and language](#content)
- [Daily use: diagnostics, timed pauses and project routing](USAGE.md)
- [FAQ and terminology](#faq)

<a id="choose"></a>

## Choose a destination

**Choose Personal DM for reminders to yourself. Choose Group / channel when a team should see them too.** A personal notification arrives in your conversation with the bot you configured, rather than a platform assistant or an official Nudilo account.

| Platform | Fields for Personal DM | Fields for Group / channel | How the DM recipient is selected |
| --- | --- | --- | --- |
| Feishu | App ID, App Secret | Group bot Webhook; signing secret if enabled | A private pairing code identifies your Open ID |
| Lark | App ID, App Secret | Group bot Webhook; signing secret if enabled | A private pairing code identifies your Open ID |
| DingTalk | Client ID, Client Secret | Group bot Webhook; signing secret if enabled | A private pairing code identifies your user ID and Robot Code |
| Telegram | Bot Token | Bot Token, group / channel ID | A pairing link or code identifies your private Chat ID |
| Slack | Bot User OAuth Token | Incoming Webhook URL | A private pairing code identifies your member ID |

You can enable several platforms at once, such as Feishu DMs and Telegram. **Each platform has one active destination**: you cannot enable both a Feishu DM and a Feishu group simultaneously, or select several Slack channels.

<a id="basics"></a>

## Preparation and common steps

### Before starting

1. Install and open Nudilo on the computer where you already use Codex or Claude Code.
2. Sign in to the receiving IM account on your phone.
3. Make sure the computer can reach the platform's APIs. Connectivity on your phone does not establish connectivity for Nudilo on the computer.
4. Organizational platforms may require permission to create, install or publish apps. If needed, ask an administrator to complete those platform settings, then enter the credentials for that app in Nudilo.
5. Prefer a dedicated notification bot so another service does not compete for pairing messages.

URLs, IDs and codes in examples below are placeholders, not usable credentials. App secrets, tokens and Webhooks act as bot passwords. Copy your own values from the platform and keep them out of public repositories, group messages and support screenshots.

### Where to start in Nudilo

1. Open **Channels** in the sidebar.
2. Select Feishu, Lark, DingTalk, Telegram or Slack.
3. If a configured summary is already displayed, select **Edit setup** first.
4. Under **Send to**, choose **Personal DM** or **Group / channel**.
5. Follow the platform-specific instructions below.
6. For a DM, pair your account before saving. Group destinations do not require account pairing.
7. Select **Save & test**, then open the destination on your phone.

### How account pairing works

1. Enter the application credentials or Bot Token, then select **Pair account**.
2. Wait for **Waiting for your code…** and a code beginning with `nudilo-`.
3. Select **Copy**. Using **the account that should receive reminders**, send the entire code as a new plain-text DM to the intended bot. Do not add quotes, commentary or a code block, or forward an old message.
4. Keep Nudilo on the current channel page. Switching to your IM app or browser is fine; navigating to another page, platform or delivery mode inside Nudilo ends the attempt.
5. Once **Account paired** appears, select **Save & test**. Pairing alone does not save a new recipient.

Each random code is valid for **three minutes**. The temporary receiver stops on success, cancellation, timeout or navigation away. Start another attempt and send its new code if necessary. The bot does not need to reply in the conversation: check the pairing status in Nudilo.

Telegram also provides an **Open bot** link; see [Telegram personal DMs](#telegram-dm).

### What the saved screen means

| Action or status | Meaning |
| --- | --- |
| Configured | A setup has been saved; this is not a guarantee that every delivery will succeed |
| App Secret / Token: Saved | The secret is stored locally and is not shown in the summary |
| Account paired | A DM recipient has been identified; a new setup still needs saving |
| Save | Store the current setup without sending a test |
| Save & test | Save first, then send one test to that destination |
| Send test | Send a test using the saved configuration |
| Edit setup | Load local values automatically, with secrets masked |
| Cancel editing | Discard this draft and restore the saved setup |

If delivery fails during **Save & test**, **a setup that was saved successfully remains saved**. Check the error in **Activity**, fix the relevant setting and test again. You do not need to enter everything again.

<a id="feishu"></a>

## Feishu

Feishu and international Lark are separate channels. Select **Feishu** in Nudilo when using a Feishu account and the `open.feishu.cn` console.

<a id="feishu-dm"></a>

### Feishu personal DMs

You need **App ID + App Secret + one pairing attempt**. There are no email, Open ID, phone-number or group-Webhook fields to fill in for this mode.

#### 1. Create an app and enable its bot

1. Sign in to the [Feishu developer console](https://open.feishu.cn/app) for your organization.
2. Create an internal custom app, labelled **企业自建应用**. Give it a recognizable name, such as `Nudilo Notifications`, and provide the required description and icon.
3. Add the **Bot / 机器人** capability from the app's capability page. For an existing app, confirm that it is enabled.
4. Find **App ID** and **App Secret** under the credentials page, labelled **凭证与基础信息**. App IDs commonly begin with `cli_`.
5. Enter both values in **Nudilo → Channels → Feishu → Personal DM**. You do not need to save yet.

Use an application bot here. A custom bot created inside group settings is a group Webhook destination and cannot perform this DM pairing. See the bot-type distinction in the [Feishu custom bot guide](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot).

#### 2. Enable the two application permissions

Search for these identifiers under **Permissions / 权限管理** and request them:

| Permission | Purpose in Nudilo |
| --- | --- |
| `im:message:send_as_bot` | Send notifications as the app's bot |
| `im:message.p2p_msg:readonly` | Receive the private verification code during pairing |

Choose permissions for the **application identity**. Nudilo's pairing does not need contact-list access or access to all group messages. The relationship between permissions and message events is documented in the [Feishu messaging overview](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1).

If the console provides a permission-import option, this minimal JSON can be used. Otherwise add the permissions individually:

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

Check whether the requested permissions are awaiting approval or publication.

#### 3. Configure the long connection and message event

1. Open **Events & Callbacks → Event Configuration**, labelled **事件与回调 → 事件配置**.
2. In Nudilo, select **Pair account** and wait for the connection to become ready.
3. In the developer console, choose the subscription method for receiving events over a **long connection / 使用长连接接收事件** and save.
4. Add **Receive message v2.0 / 接收消息 v2.0**, with event identifier **`im.message.receive_v1`**.
5. Save the event settings.

**Why start pairing first?** Feishu checks for a live connection when saving this subscription method. Nudilo opens its receiver only during pairing, so save the console setting while that three-minute attempt is waiting; start again if it expires. A public server or callback URL is unnecessary. This sequence follows Feishu's [long-connection requirements](https://open.feishu.cn/document/ukTMukTMukTM/uYDNxYjL2QTM24iN0EjN/event-subscription-configure-).

Configure the message event under Event Configuration. Nudilo does not require card-button callbacks, Verification Token or Encrypt Key.

#### 4. Publish and make the app available to yourself

1. Open **Version Management & Release / 版本管理与发布** and create a version with a version number and release description.
2. Include the receiving account in the app's availability range.
3. Submit the version and complete any required administrator approval.
4. When changing permissions, bot features or events later, publish again when the console requests it.
5. Search for the bot in the Feishu client and open its DM. If it cannot be found, check the organization, availability and publication status.

Availability affects whether the bot can message a user; see the [Feishu send-message API](https://open.feishu.cn/document/server-docs/im-v1/message/create).

#### 5. Pair and send a test

1. Return to Nudilo and start a new pairing attempt if the earlier one ended.
2. Copy the current code and send it privately to the application bot.
3. Wait for **Account paired**.
4. Select **Save & test**.
5. Open that bot's conversation on your phone and look for the connection-test message containing `Nudilo` and `Codex`.

### Feishu groups

1. Open the intended group in Feishu.
2. In group settings, open the bot list and add a **Custom Bot / 自定义机器人**. Ask the group owner or administrator if you lack permission.
3. Name the bot, complete creation and copy its full Webhook URL.
4. Choose **Feishu → Group / channel** in Nudilo and paste it into **Webhook URL**.
5. Match the security settings below, then select **Save & test**.
6. Check this group for the test. The Webhook belongs to the group where the bot was created.

The expected URL shape is:

```text
https://open.feishu.cn/open-apis/bot/v2/hook/<YOUR_WEBHOOK_ID>
```

| Bot security setting | What to do |
| --- | --- |
| Signature verification | Expand **Signing (optional)** and paste the bot's signing secret |
| Custom keywords | Allow both `Codex` and `Claude Code`; tests contain both names, real notifications contain their source |
| IP allowlist | Have the administrator allow the computer's actual public outbound IP; review it after changing networks or proxies |

Meet every enabled check. Once signing is enabled on the platform, its secret cannot be left blank in Nudilo. This signing secret is separate from a DM app's App Secret. See the [Feishu custom bot security rules](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot).

### Feishu troubleshooting

| Symptom | Check in this order |
| --- | --- |
| Console cannot detect a long connection | Start pairing in Nudilo, wait for readiness, then save the console setting while it is waiting |
| Bot is visible but has no message input | Bot capability, private-message permission, message event and published version; reopen the conversation |
| Code was sent but pairing keeps waiting | Correct application bot, DM rather than group, current code, and no competing receiver using the same app |
| Pairing succeeds but sending fails | Receiving permission is separate from sending: check `im:message:send_as_bot`, publication approval and account availability |
| Group keyword rejection | Allow both `Codex` and `Claude Code` with the matching spelling and case |
| Group signature rejection | Copy the signing secret again, make sure it is not App Secret, and check the computer's clock |
| Webhook validation fails | Use the full Feishu v2 group-bot URL, not a Lark URL, app ID or console page |

<a id="lark"></a>

## Lark

Use the [Lark Developer Console](https://open.larksuite.com/app) and `open.larksuite.com` endpoints. Feishu credentials and Webhooks do not belong in the Lark channel.

### Lark personal DMs

1. Sign in to the console and create a **Custom App** in the intended organization.
2. Add **Bot** under **Add App Capability / Features**.
3. Copy App ID and App Secret from **Credentials & Basic Info** into **Nudilo → Lark → Personal DM**.
4. Under **Permissions & Scopes**, request application permissions `im:message:send_as_bot` and `im:message.p2p_msg:readonly`. They enable sending notifications and receiving the pairing code respectively.
5. Open **Events & Callbacks → Event Configuration**, then select **Pair account** in Nudilo.
6. When Nudilo is connected, select reception over a **long connection** in the console and save.
7. Add the **Receive message** event **`im.message.receive_v1`** and save.
8. Create and publish a version under **Version Management & Release**, include your receiving account in **Availability**, and complete required approval.
9. Find the application's bot in Lark and open its DM.
10. Start a fresh pairing attempt in Nudilo, send its code to that DM, then select **Save & test** once identified.

Lark's [app and bot setup example](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/mcp_integration/develop-mcp-intelligent-assistant-bot) helps locate the console features. That example covers additional products: Nudilo only needs the two permissions and message event listed above, not its full permission set or deployment steps.

If the console cannot detect a connection when first saving, keep a Nudilo pairing attempt waiting and save again; restart pairing after a timeout. Exact English labels follow the current console. Nudilo does not need your email, Open ID, an OAuth callback URL or an additional local script.

If the published app cannot be found, check the current organization and account availability. See [Lark messaging FAQs](https://open.larksuite.com/document/server-docs/im-v1/faq) for visibility and bot-input issues.

### Lark groups

1. In the target group, open **Settings → Bots → Add Bot → Custom Bot**.
2. Name and create the bot, then copy its Webhook.
3. In Nudilo, select **Lark → Group / channel** and fill in **Webhook URL**.
4. If **Signature verification** is enabled, expand **Signing (optional)** and enter that signing secret.
5. For keyword checks, allow both `Codex` and `Claude Code`. For an IP allowlist, allow the computer's public outbound IP.
6. Select **Save & test** and inspect the target group.

The group creation path is shown in the [Lark Help Center](https://www.larksuite.com/hc/en-US/articles/897957711856-http-requests-sending-group-messages-applicable-for-external-groups); security options are covered by the [Lark custom bot guide](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/bot-v3/use-custom-bots-in-a-group).

The expected Webhook format is:

```text
https://open.larksuite.com/open-apis/bot/v2/hook/<YOUR_WEBHOOK_ID>
```

### Lark troubleshooting

| Symptom | What to check |
| --- | --- |
| Correct-looking credentials fail | They must come from Lark, with Lark selected in Nudilo; check desktop API connectivity |
| Subscription settings cannot be saved | Establish Nudilo's pairing connection first, then save the long-connection setting |
| No pairing response | Private-message permission, `im.message.receive_v1`, published version, current code and competing receivers |
| Pairing works but delivery fails | Sending permission, administrator approval, availability, and whether credentials were changed |
| Group delivery fails | Webhook domain, signing secret, keyword and IP allowlist |

<a id="dingtalk"></a>

## DingTalk

### DingTalk personal DMs

You need **Client ID (AppKey) + Client Secret (AppSecret)**. Nudilo extracts your organization user ID and Robot Code from the pairing message automatically.

#### 1. Create an internal application

1. Sign in to the [DingTalk developer console](https://open-dev.dingtalk.com/) and select the receiving account's organization.
2. Create an internal DingTalk application under application development. Use the application-based route, then add a bot capability to it.
3. Complete the app name, description and other required fields.
4. Copy **Client ID** and **Client Secret** from the credentials page. Older interfaces may call them **AppKey** and **AppSecret**; they map to the same two Nudilo fields.
5. Select **DingTalk → Personal DM** in Nudilo and enter both values.

The official [bot application tutorial](https://open-dingtalk.github.io/developerpedia/docs/explore/tutorials/stream/bot/java/create-bot/) shows the creation route and old/new credential names.

#### 2. Enable the bot and permissions

1. Enable the bot from the app's capability page or **机器人与消息推送** and complete its required profile.
2. Set its message receiving mode to **Stream** and save.
3. Under permission management, find the robot category and grant the permission for sending private messages. Search for **企业内机器人发送消息**; the console may show the identifier `qyapi_robot_sendmsg`.
4. Create and publish an app version, include your account in the visible/available range and complete any approval.

See DingTalk's [permission application instructions](https://open.dingtalk.com/document/orgapp/apply-for-permissions-8). The `qyapi_robot_sendmsg` setting also appears in [Alibaba Cloud's official DingTalk setup](https://help.aliyun.com/zh/starops/user-guide/connect-to-dingtalk). Its card permissions serve a different product and are not required for Nudilo's text notifications.

#### 3. Pair and test

1. Search for the bot in DingTalk. If results have categories, check **功能 / Functions**, then open the bot's DM.
2. Select **Pair account** in Nudilo and wait for the code.
3. Send it privately from the receiving account in the same organization. An `@bot` message in a debugging group does not pair a personal destination.
4. Once Nudilo shows the account as paired, select **Save & test**.
5. Check the bot's DM on your phone.

The search route is shown in DingTalk's [official bot interaction guide](https://open-dingtalk.github.io/developerpedia/docs/explore/tutorials/stream/bot/python/test-bot/). Nudilo currently requires the sender and bot to belong to the same organization; it uses [BatchSendOTO](https://open-dev.dingtalk.com/apiExplorer#/?devType=org&api=robot_1.0%23BatchSendOTO) with one identified recipient.

Nudilo establishes Stream reception during pairing. You do not need a public receiving URL or to install an SDK. See the [official DingTalk Stream SDK](https://github.com/open-dingtalk/dingtalk-stream-sdk-go) for the underlying mechanism.

### DingTalk groups

Nudilo currently sends DingTalk group notifications through a **custom group-bot Webhook**, not the enterprise application's group-message API.

1. Open the target group's settings and its bot-management page, usually **群设置 → 机器人 → 添加机器人**; some clients call it **智能群助手**.
2. Select a custom bot, name it and configure the required security options. For an existing bot, open its settings.
3. Copy the complete Webhook, including everything after `access_token=`.
4. In Nudilo, select **DingTalk → Group / channel** and fill in **Webhook URL**.
5. If signing is enabled, expand **Signing (optional)** and paste the bot's `SEC…` secret.
6. For keyword validation, allow both `Codex` and `Claude Code`; for an IP allowlist, check the computer's public outbound IP.
7. Select **Save & test** and inspect that group.

See [DingTalk custom bot creation](https://open.dingtalk.com/document/orgapp/custom-bot-creation-and-installation) and [security settings](https://open.dingtalk.com/document/orgapp/customize-robot-security-settings). Outgoing message reception is unnecessary for notifications.

```text
https://oapi.dingtalk.com/robot/send?access_token=<YOUR_ACCESS_TOKEN>
```

Do not enter an AppKey, Robot Code, temporary `sessionWebhook` or API debugging URL here. Nudilo calculates signatures; do not append your own `timestamp` or `sign` parameters.

If your group or organization has no custom-bot entry, ask an administrator whether it is supported there. Enterprise app credentials cannot replace a Webhook. This version does not expose enterprise-app group messaging; use DingTalk personal DMs or another available group platform instead.

### DingTalk troubleshooting

| Symptom | Check in this order |
| --- | --- |
| Bot cannot be found | Current organization, publication, availability and enabled bot capability |
| Stream connection fails | Client ID and Secret from the same app, with Stream mode saved |
| A code in a group does not pair | Personal pairing accepts DMs; open the bot's private conversation and send a fresh code |
| User or Robot Code cannot be identified | Use an account in the same organization, confirm publication and start pairing again |
| Pairing works but private delivery fails | Sending permission and availability; receiving a message does not grant proactive sending permission |
| Group notification rejected | `access_token`, signing secret, `Codex` / `Claude Code` keywords, IP allowlist and computer clock |

<a id="telegram"></a>

## Telegram

Telegram DMs, groups and channels all use a **Bot Token**. Only the destination differs: DMs are paired automatically, while groups and channels require an address.

### Create your bot

1. Open the official [@BotFather](https://t.me/BotFather) in Telegram.
2. Send `/newbot` and provide a display name when asked.
3. Choose an unused bot username ending in `bot`, such as `your_name_nudilo_bot`.
4. Copy the token BotFather returns, including its numeric prefix, colon and entire remaining string.
5. Select **Telegram** in Nudilo and fill in **Bot Token**.
6. Keep the bot's `@username` handy for opening its DM or adding it to a group. The username is not the token.

See the [official Telegram tutorial](https://core.telegram.org/bots/tutorial#obtain-your-bot-token) and [BotFather reference](https://core.telegram.org/bots/features#botfather). Nudilo already implements sending; you do not need to continue into the tutorial's programming steps.

<a id="telegram-dm"></a>

### Telegram personal DMs

1. Choose **Personal DM** and confirm the Bot Token is entered.
2. Select **Pair account** and wait for the code and **Open bot** link.
3. Follow **Open bot** and press **Start** in Telegram. This link includes the current pairing code.
4. If Start is unavailable or pairing does not complete, copy the code from Nudilo and send it as a new message directly to your bot.
5. After **Account paired** appears, select **Save & test**.

Message **your own bot**, not BotFather. A plain `/start` can open a conversation but is insufficient for Nudilo pairing without the current code. You do not need to look up your private Chat ID or enter your personal `@username` as a group/channel address.

### Telegram groups

1. Add the bot to the target group. If it cannot be invited, check in BotFather that the bot is allowed to join groups.
2. Allow it to send text messages. Full administrator privileges are normally unnecessary in a group.
3. Obtain the group's Chat ID using the instructions below.
4. Select **Group / channel** in Nudilo.
5. Enter the Bot Token and the complete negative **Group / channel ID**, for example `-1001234567890`.
6. Select **Save & test** and check the group.

#### Find an ID without a third-party lookup bot

This procedure reads Telegram's own Bot API and is suitable for a new, dedicated notification bot:

1. Cancel any active pairing attempt in Nudilo. Do not simultaneously run another program reading updates for this bot.
2. In the target group, send a command explicitly addressed to it, such as `/nudilo_setup@your_name_nudilo_bot`. Replace the example username with your bot's username, without adding a second `@`.
3. In a private browser window's **address bar**, open the URL below after replacing the whole `<YOUR_BOT_TOKEN>` placeholder. Do not put the token-bearing URL into a search box or share it.

   ```text
   https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates
   ```

4. Locate your new message in the response. Confirm that `chat.title` matches the target group and read **`message.chat.id`**.

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

5. Copy the entire negative number, including `-`, into Nudilo. Do not copy `from.id`, `message_id` or `update_id`.
6. If `result` is empty, send a fresh command and refresh. For a Webhook conflict, use a dedicated bot rather than deleting another service's Webhook.
7. Close the private window afterwards. No `offset`, `deleteWebhook` or `drop_pending_updates` parameter is needed.

Privacy mode allows commands explicitly addressed to the bot, so it normally need not be disabled for this lookup. See the [Telegram Bot FAQ](https://core.telegram.org/bots/faq#what-messages-will-my-bot-get) for reception rules and [getUpdates](https://core.telegram.org/bots/api#getupdates) for response fields.

### Telegram channels

**Public channel:**

1. Add the bot as a channel administrator with permission to publish messages.
2. Select **Group / channel** in Nudilo and enter its Bot Token.
3. Use the channel's public username, such as `@your_nudilo_alerts`, as **Group / channel ID**. Do not enter a display name or `https://t.me/…` link.
4. Save, test and inspect the channel.

**Private channel:**

1. Add the bot with permission to publish, as above.
2. Publish a new ordinary post yourself in the channel.
3. Use the official `getUpdates` lookup above to find **`channel_post.chat.id`**, verify the channel name and copy the complete negative number.
4. Enter that ID in Nudilo, save and test. There is no need to make a private channel public to obtain its ID.

Nudilo currently has no topic-ID or `message_thread_id` field, so it cannot target a specific forum-group topic. Telegram's destination rules are documented under [sendMessage](https://core.telegram.org/bots/api#sendmessage).

### Telegram troubleshooting

| Symptom | Action |
| --- | --- |
| Invalid token format | Copy the complete BotFather token; do not use a username or bot link |
| Start did not complete pairing | Use this attempt's **Open bot** link or send its code directly; plain `/start` has no verification code |
| Webhook or competing update-reader conflict | Use a dedicated bot; Nudilo does not remove existing Webhooks |
| Too many pending updates | Pairing does not consume unrelated updates; a dedicated bot avoids an old backlog hiding the new code |
| Group/channel unavailable or delivery forbidden | Check the full ID, bot membership and sending rights; channels need publishing permission |
| Old ID fails after a group upgrade | Retrieve the current group ID and save it |
| Phone Telegram works but Nudilo times out | Check desktop access to `api.telegram.org` and any proxy; phone access is independent |

<a id="slack"></a>

## Slack

Slack personal DMs use a bot OAuth token. Channel notifications use an Incoming Webhook. The two credential types are not interchangeable.

### Slack personal DMs: a new app

#### 1. Use Nudilo's app manifest

1. Open **Slack → Personal DM** in Nudilo. If already configured, choose **Edit setup** first.
2. Expand **Setup help** and select **Copy app manifest**. Alternatively, copy the full [JSON file in this repository](../internal/desktop/slack-manifest.json).
3. Open [Slack app management](https://api.slack.com/apps) and choose **Create New App → From a manifest**.
4. Select the destination workspace and paste the JSON into the manifest editor. Choose JSON if the editor offers JSON/YAML tabs.
5. Review the configuration and create the app.

The supplied manifest configures three bot permissions, enables the Messages tab and user input, and disables Socket Mode and Token Rotation, which this Nudilo setup does not use. See Slack's [app-manifest guide](https://docs.slack.dev/app-manifests/configuring-apps-with-app-manifests/) for the creation flow.

#### 2. Install the app and copy the token

1. Open **OAuth & Permissions** in the Slack app console.
2. Select **Install to Workspace** and authorize the installation. Complete workspace administrator approval if required.
3. Copy the resulting **Bot User OAuth Token**, beginning with **`xoxb-`**.
4. Paste it into the matching field in Nudilo.

Do not use Client Secret, Signing Secret, an `xapp-` app-level token or an `xoxp-` user token. Nudilo accepts the bot token for this mode; no OAuth Redirect URL, Socket Mode or event callback server is needed.

#### 3. Open the app and pair

1. In the correct Slack workspace, find the newly created app under **Apps**.
2. Open its **Messages** tab and confirm that you can type a message.
3. Select **Pair account** in Nudilo and wait for this attempt's code.
4. From the receiving member account, send the code as a new plain-text message to this app. Do not send it to Slackbot, a channel or a conversation with another person, or edit an old message to contain the code.
5. Once paired, select **Save & test**.
6. Open the conversation with the app on your phone and check the test.

During pairing, Nudilo uses [conversations.list](https://docs.slack.dev/reference/methods/conversations.list/) to discover the bot's DMs and [conversations.history](https://docs.slack.dev/reference/methods/conversations.history/) to find the current code. Delivery uses [chat.postMessage](https://docs.slack.dev/reference/methods/chat.postMessage/). This implementation therefore does not need Socket Mode or Event Subscriptions.

### Slack personal DMs: an existing app

Check these permissions under **OAuth & Permissions → Scopes → Bot Token Scopes**:

| Bot scope | Purpose in Nudilo |
| --- | --- |
| `chat:write` | Send notifications as the bot |
| `im:read` | Find the bot's private conversations during pairing |
| `im:history` | Read the new pairing code |

Then:

1. Enable **Messages Tab** under **App Home**.
2. Allow users to message the app. Some console labels also mention Slash Commands; you do not need to create a Slash Command.
3. After changing permissions, reinstall the app from **OAuth & Permissions** so the workspace grants them.
4. Enter its current Bot User OAuth Token in Nudilo, pair again, save and test.

Messages-tab settings are covered in [Slack App Home](https://docs.slack.dev/surfaces/app-home/). Nudilo does not need history access for all public or private channels. This version does not implement automatic Slack OAuth token rotation; use the supplied manifest's configuration.

### Slack channels

1. In [Slack app management](https://api.slack.com/apps), create an app **From scratch**, name it and choose its workspace, or use an existing app.
2. Under **Incoming Webhooks**, turn on **Activate Incoming Webhooks**.
3. Select **Add New Webhook to Workspace**, choose the target channel and authorize it.
4. To select a private channel, you must already be a member of it.
5. Copy the generated Webhook URL.
6. Choose **Slack → Group / channel** in Nudilo, paste it into **Webhook URL**, then select **Save & test**.

See the official [Slack Incoming Webhooks guide](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/) for creation and channel authorization.

```text
https://hooks.slack.com/services/<WORKSPACE>/<WEBHOOK>/<SECRET>
```

Slack chooses the destination when you authorize the Webhook. To change channels, create the corresponding Webhook and save it in Nudilo. This mode needs no Bot Token, channel ID, Signing Secret or personal pairing.

Personal DMs currently support ordinary Slack workspaces; Slack Gov DMs are not implemented. Group Webhook validation accepts both `hooks.slack.com` and `hooks.slack-gov.com`.

### Slack troubleshooting

| Symptom | Check in this order |
| --- | --- |
| No Bot User OAuth Token | Install the app in the correct workspace, then check OAuth & Permissions |
| Cannot type in Messages | Enable Messages Tab and allow messages; reopen the app conversation |
| `missing_scope` or missing permissions | Check all three Bot Scopes and reinstall before pairing again |
| Invalid, expired or revoked token | Obtain a valid bot token; after replacing it, pair again and save in Nudilo |
| Code never recognized | Correct workspace, app and current code; send a new DM rather than editing or forwarding a message |
| Many bot DMs or rate limiting | Wait before retrying; use a dedicated personal-notification app to reduce the scan |
| Channel test fails | Check the Webhook destination, whether the channel is archived and whether the app is still allowed |
| Trying to enter `#channel-name` | This mode requires the channel's Webhook URL, not its name |

<a id="verify"></a>

## Verify automatic notifications and phone alerts

### Layer 1: can the platform configuration send?

1. Select **Save & test** in the form, or **Send test** in the saved summary.
2. Open **Activity** and check whether the platform accepted the test. Resolve the recorded error if it failed.
3. Locate the message in the destination conversation. Check that you are looking at the right organization, account, bot, group or channel.

A successful test confirms platform acceptance. It does not guarantee a phone sound, banner or lock-screen alert.

### Layer 2: does the phone alert you?

1. Check the phone's system notification permissions for the IM app, including sound and lock-screen display.
2. Check whether the conversation is muted or the channel only alerts for mentions.
3. Review the IM's notification pause, working-hours and mobile-alert behavior while its desktop client is active. Available options and labels differ by platform.
4. Send another test with the phone locked or another app in the foreground to confirm the actual alert.

Nudilo does not force attention with an `@everyone` mention. This matters especially for groups configured to notify only on mentions. First confirm the message exists in the conversation, then investigate phone display settings.

For example, Feishu can suppress mobile alerts while a desktop or iPad client is online, and Slack provides separate mobile notification preferences. See [Feishu notification settings](https://www.feishu.cn/hc/en-US/articles/111093338362-set-up-message-notifications) and [Slack notification settings](https://slack.com/intl/en-gb/help/articles/201355156-Configure-your-Slack-notifications).

### Layer 3: does a real assistant event trigger delivery?

1. In Nudilo's **Overview**, confirm reminders are enabled and the desired completion, question and approval types are selected.
2. Refresh the originating assistant’s association status; reconnect if instructed.
3. Let Codex or Claude Code finish a new, short task and look for a new turn-ending event in Activity.
4. Confirm the phone received that real event rather than only the earlier connection test.

Already-open Codex chats can produce turn-ending reminders through the local completion monitor. Chats opened before listener installation need reopening to load question and approval events. Reopen existing Claude Code sessions after association if listeners are not loaded.
Requirements and event coverage are in [Behavior and data](BEHAVIOR.md#claude-code).

A turn ending means that round of output ended, not that the entire task succeeded. Nudilo currently sends reminders only; answer, provide information or approve actions in the originating assistant on the computer.

### Locate the failing stage

| Symptom | First place to check |
| --- | --- |
| Pairing fails | Credentials, receiving permissions/mode and current code |
| Pairing succeeds, test fails | Sending permissions, publication, recipient availability and network |
| Test succeeds, conversation appears empty | Correct account, organization, bot, group or channel |
| Message exists, no phone banner | System notifications, conversation mute and IM push settings |
| Test works, real task has no Activity entry | Reminder toggle/types, the assistant’s association and loaded chat events |
| Real event appears but delivery fails | Recorded platform error and that channel's setup |
| Activity says Skipped | Whether reminders were paused, the type disabled or channels unconfigured when it arrived |
| Nothing arrives after quitting Nudilo | Nudilo must remain running; on macOS closing the window leaves it in the menu bar, while Quit stops it |

<a id="manage"></a>

## Edit, pair again or remove a destination

### Edit a saved setup

1. Open the platform and select **Edit setup**.
2. Saved values are filled in automatically. Masked secrets are not empty values.
3. Make the change. If App ID, App Secret or Bot Token changes, pair the DM account again.
4. Select **Save & test**, or **Cancel editing** to discard the draft.

### Choose a different personal recipient

1. Edit the configuration and select **Pair again**.
2. Send the new code to the same bot from the new receiving account.
3. Save after identification. The new recipient replaces the old one; it does not add a second DM destination.
4. Failed or cancelled pairing leaves the saved recipient unchanged.

### Switch between DM and group mode

Choose the other mode while editing, fill in or review its fields, then save. The other mode's setup remains in the local credential file, but only the saved mode is active. Telegram shares one Bot Token between both modes; after replacing it, check whether each destination is accessible to the new bot.

Changing destinations does not forward old pending retries to the new recipient. Subsequent new events use the new saved setup.

### Remove a channel or rotate credentials

**Remove channel** asks for confirmation, then deletes that platform's local configuration and disables it in Nudilo. It does not delete a bot, uninstall an app or revoke a token on the platform. Use the platform console separately if you intend to retire the bot.

After resetting a secret, token or Webhook on the platform, update the corresponding Nudilo field. Pair DMs again after changing application credentials. There is no keychain-loading or old-credential import step.

<a id="content"></a>

## Customize notification text and language

Customization is optional: existing installations continue using the default
messages. Feishu, Lark, DingTalk, Telegram and Slack share one set of templates
for each language, for both personal messages and groups/channels. The same
channel configuration serves both assistants; personal DMs need no new pairing.
Group bots with keyword checks must allow both `Codex` and `Claude Code`.

1. Open **Settings → Notification content → Customize**. The language shown is
   the app's current language.
2. Edit **Notification title** (one line, up to 80 characters). The title is
   shared by all reminder types; the source (`Codex` or `Claude Code`) and event type are always appended,
   so a completion cannot be confused with a question or approval request.
3. Select **Turn ended**, **Answer needed**, **Approval needed** or **Interrupted**, then edit
   **Message text** (up to 1,000 characters per type).
4. Place the cursor in the body and click **Assistant**, **Project**, **Time** or **Summary** to
   insert a variable, replacing any selected text. The preview updates as you
   type and uses example data; switch between Codex and Claude Code above it.
   The preview does not send a message.
5. Select **Save** to apply all four types and the title for this language.
   **Cancel** discards this language's unsaved changes. **Restore defaults**
   resets this language's draft; select **Save** to apply the reset.

| Variable | Replaced with |
| --- | --- |
| `{agent}` | `Codex` or `Claude Code`, according to the event source |
| `{project}` | Project folder name, without the full path |
| `{time}` | Event time, formatted in the notification language and computer's time zone |
| `{summary}` | Up to 500 characters of the available assistant summary, only when opted in |

Variables are supported in the body. Other brace expressions are rejected with
an error below the input. Empty titles or bodies use the language's defaults.
Long expanded messages are shortened automatically; the preview indicates when
its example would be shortened. Actual length depends on the project and summary.

**Summary stays off by default.** Enable **Settings → General → Include a summary**
to use its insertion button. If the option is off, existing `{summary}` variables
also expand to nothing. Some events have no summary, including secret-input
questions. Removing the variable omits the summary from that template; the
general opt-in still controls local summary collection. Claude Code MCP input
requests and API failures do not include a content summary. Existing custom
text is preserved; replace a hard-coded assistant name with `{agent}` if desired.

### How Chinese and English are selected

- Choose **简体中文** in **Settings → Appearance → Language** for Chinese event
  labels, default text and dates. Choose **English** for English.
- **Follow system** uses the app's resolved system language, with English as the
  fallback for unsupported languages.
- Chinese and English custom text is saved independently. Switch the app language
  to edit the other version. If that version has not been customized, its own
  default messages are used; it does not reuse the other language's custom text.
- Project names, summaries and your literal custom text are not translated
  automatically. Each language's unsaved draft survives an in-app language switch.
- New sends and retries use the language, saved template and summary setting in
  effect when each send starts. Messages already sent remain unchanged.
- **Send test** uses the current title and language with a fixed connection-test
  body. Use the editor preview to check the four event templates, or confirm
  them with a new real Codex or Claude Code event after saving.

<a id="faq"></a>

## FAQ and terminology

### Where is configuration stored?

Application secrets, tokens, Webhooks and recipient identifiers are stored in an encrypted local credential file, without macOS Keychain or another OS credential store. Nudilo automatically reads saved local values when editing. See [local data](BEHAVIOR.md#local-data) for file locations and backup details.

### Why is receiving permission needed if I only want notifications?

It lets Nudilo identify your account from a verification code instead of asking you to look up an internal user ID. The temporary receiver closes after pairing. Nudilo does not treat everyday DMs as assistant instructions or persist the incoming chat content. Receiving permissions are needed again for future pairing attempts.

### Can I reply to Codex or Claude Code from my phone?

Not in this version. Bot messages are reminders; replying does not forward an answer to either assistant or approve an action.

### What happens when I close the window, sleep or quit?

- Closing the main window keeps Nudilo running when a system tray is available. On macOS it also hides the Dock icon and retains the menu bar icon.
- Sleep or loss of connectivity prevents immediate delivery. Queue processing resumes according to record state when the computer and connection return.
- Quitting Nudilo stops its notification service. Enable launch at login in Settings if you want it to start when you sign in.

### Field glossary

| Term | Meaning | Where it comes from |
| --- | --- | --- |
| App ID / Client ID / AppKey | Application identifier | Enterprise platform's app credentials page |
| App Secret / Client Secret / AppSecret | Secret paired with that application ID | The same app's credentials page |
| Bot Token | Credential for acting as the bot | Telegram BotFather or Slack OAuth page |
| Webhook URL | Complete delivery address for a fixed group or channel | Group bot settings or Slack Incoming Webhooks |
| Signing secret | Secret used to verify group-bot requests | Feishu, Lark or DingTalk bot security settings |
| Open ID / user ID / member ID | Internal identifier for a DM recipient | Discovered by Nudilo pairing; no manual entry |
| Chat ID | Telegram conversation identifier | Paired automatically for DMs; looked up for groups and private channels |
| Stream / long connection | Temporary connection used to receive a pairing message | Enabled in the platform, established by Nudilo |
| Availability / visible range | Organization members allowed to use an app | Enterprise app publication settings |

For further troubleshooting, record the platform, destination mode, whether failure occurred during pairing or delivery, and the error shown in Activity. Redact secrets, tokens, Webhooks and personal recipient identifiers from reports.
