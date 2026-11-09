# Everyday use

[English](USAGE.md) · [简体中文](USAGE.zh-CN.md) · [README](../README.md)

## Finish setup once

Overview offers three steps: detect an AI assistant, connect a notification channel,
and receive a test message. Only installed, previously used or explicitly added
assistants are shown. **Add assistant** reveals the other supported assistants.
You only need Codex or Claude Code, not both.

The guide disappears when all three steps have evidence of completion. **Later**
hides it; **Settings → Show setup guide** brings it back. A successful channel test
checks the sending side. Run a real turn in your assistant to verify event collection
as well. Platform acceptance does not establish that your phone displayed an alert.
See the [platform setup guide](CHANNELS.md) for permissions and mobile notifications.

## Check whether reminders are working

Overview and the sidebar use the same overall status. A working assistant and at
least one saved channel can keep running while another channel needs attention.
An unused assistant does not prevent the app from being ready. Recent real events
appear above channel cards; use **View all** for full activity. Channel cards show
the logo, name and configuration status. Hover a saved channel card for its latest
delivery status.

Open a saved channel to see its destination, most recent delivery state and any
available evidence of a real AI event reaching the platform. Test messages are
identified separately. **Configuration details** expands the saved credential
summary; **Edit setup** opens the existing values, with secrets masked. **Send test**
sends to that channel, independently of project routing and pause settings.
**Remove channel** asks for confirmation.

Delivery evidence comes from retained activity for the current saved setup.
Saving the setup again resets that evidence; clearing activity clears it too.
“Not tested yet” means there is no retained proof, not that the saved setup is invalid.

**Overview → Diagnose connection** checks assistant listeners, the local event
queue and local credentials. It continues through failures so you can see all
results together. It does not send a message, start a model turn, repair hooks or
contact IM APIs. **AI assistant settings** opens the relevant connection controls;
**Reconnect** repairs listener setup. **Test a channel** opens a channel where you
can explicitly send a test. Network and IM permission failures require that test
or a real event to verify. Diagnostic results reflect the last check; run diagnostics
again after fixing a local storage issue to refresh its warning.

## Pause for a while

Use the dropdown arrow beside **Pause reminders** (labeled **Pause duration**),
or **Pause** in the tray/menu bar, and choose 15 minutes, one hour, tomorrow at 09:00, or until you resume manually.
The tomorrow option uses the computer's local time at the moment you select it.
**Pause reminders** pauses indefinitely; **Enable reminders** resumes early.
The current deadline is visible in Overview and the tray menu.

The deadline is saved. Nudilo resumes when the deadline arrives while running,
or on the next run/wake if the computer was asleep or the app was closed. Events
generated during the recorded pause interval are skipped, including when they
are read from the queue after waking. Within the same running process, existing
pending deliveries are held during the pause and remain subject to the normal
24-hour expiration. Quitting and reopening skips those old pending deliveries.
A send already underway may finish. Explicit channel tests remain available while paused.

## Close the window or quit

Closing the window leaves Nudilo in the menu bar/system tray, where reminders
continue. **Quit Nudilo** stops the app. On the next launch, notifications generated
before startup and retries left from the previous run appear as **Skipped** in
Activity with an explanation. They are not automatically sent to your phone.
New notifications send normally. This also applies after restarting to update.
If the computer only sleeps and the app stays running, normal retry, pause and
24-hour expiration rules continue when it wakes.

## Send different projects to different channels

This is optional: without rules, new notifications go to every configured channel.
To add a rule:

1. Configure and test the destination channels first.
2. Open **Settings** and scroll to **Project routing**.
3. Expand **Choose channels by project**, then choose **Add rule**.
4. Enter the exact project name shown in Activity, or choose **Use a recent project**.
5. Choose all assistants, Codex or Claude Code, then select one or more configured channels.
6. Choose **Save rule**.

Names match exactly, including case. They are folder names already carried in
notifications, not full paths. Projects in different folders with the same name
share a rule. A rule restricted to one assistant takes precedence over the same
project's all-assistant rule. Duplicate project/assistant rules are rejected.

A matching rule never falls back to unrelated channels. If its destination has
been removed or is unavailable, the rule remains visible and the affected
notification is skipped or records a delivery error. Other selected, working
channels can still receive it. Edit the rule to repair the destination.

Changing a rule does not forward an existing notification to newly selected
channels. Pending sends to channels no longer selected are skipped when processed.
Removing a rule requires confirmation; future events then use another matching
rule, or all configured channels if none matches. Channel tests always target the
channel you explicitly test. Each platform still has only one active destination.

## Find a notification

Activity groups records by local date. Search project names (case-insensitive
substring matching) and filter by assistant or delivery status; filters combine.
A status filter also matches an individual channel in a partially delivered event.
Test messages are excluded when a specific assistant is selected.

Expand **Delivery details** to see channel errors, retry times or a skip reason.
**Retry failed channels** is available for failed real events. Successful channels
are not sent the same event again. The initial list shows up to 100 matching
records; **Show more** reveals more retained records.

**Clear all** requires confirmation. It clears visible delivery evidence and
cancels pending retries, while retaining minimal event identities to prevent
replaying old events. See [Behavior and data](BEHAVIOR.md) for retention details.

## Keep settings accessible

Scroll through Settings to reach General, Notification types, Notification content,
Project routing, each assistant, Appearance and Updates. Update indicators and tray
actions open Updates directly.

Use Tab/Shift+Tab to move between controls, Space to toggle switches and Enter
for actions. Menus and select controls support arrow keys; Escape closes them.
System text scaling and high-contrast preferences are applied where exposed by
the native runtime; reduced-motion preferences disable the reminder animation.
English and Simplified Chinese cover the guide, diagnostics, rules, pause options
and activity filters. Outgoing message language continues to follow the app's
selected or resolved system language.
