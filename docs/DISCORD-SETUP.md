# ReelPing — Discord Setup

ReelPing uses **Discord incoming webhooks**. There is no bot to invite, no token
to manage, and no gateway connection. One webhook posts messages to one channel.

## Create a webhook

1. In Discord, open the **server** and channel you want alerts in.
2. **Server Settings → Integrations → Webhooks → New Webhook** (or *View
   Webhooks → New Webhook*).
3. Choose the target **channel** and, optionally, a name/avatar.
4. Click **Copy Webhook URL**. It looks like:

   ```
   https://discord.com/api/webhooks/123456789012345678/aBcDeF...secret
   ```

5. In ReelPing (setup wizard or Settings → Discord), paste it and click **Send
   test**. You should see “ReelPing test successful” in the channel.

> The webhook URL is a **secret** — anyone with it can post to your channel.
> ReelPing stores it redacted and never shows, logs, or exports it. Treat it
> like a password; rotate it in Discord if it leaks.

## Mentions (ping safety)

Every ReelPing message sends an explicit `allowed_mentions` object, so mentions
only fire when you choose them:

| Policy | Behaviour |
| --- | --- |
| **No mention** (default) | No ping. Even literal `@everyone` text in a field cannot notify anyone. |
| **@here** | Notifies currently-online members. |
| **@everyone** | Notifies all members. Requires explicit selection + confirmation. |
| **Specific role** | Notifies one role you specify by numeric role ID. |

- `@everyone`/`@here` are **never** enabled during first-run setup and are never
  inherited by automatic alerts unless you configure them separately.
- Automatic **outage** and **recovery** mention policies are configured
  independently of manual announcements (Settings → Discord).
- ReelPing warns you before a test that might actually ping members.

### Finding a role ID

Enable **Developer Mode** (Discord Settings → Advanced), then right-click a role
→ **Copy Role ID**. Paste the numeric ID into ReelPing. ReelPing validates it is
numeric and only allows that exact role to be pinged.

## Message styles

ReelPing sends rich embeds with a status symbol and colour (symbol always
present, so colour is never the only signal):

- 🔵 Information · 🟡 Maintenance · 🟠 Degraded · 🔴 Outage · 🟢 Recovery

All user-entered text is escaped and length-limited to Discord's documented
embed limits (title 256, description 4096, field value 1024, 25 fields, 6000
total characters).

## Delivery reliability

On transient errors ReelPing retries with exponential backoff and jitter, honours
Discord's `429` `retry_after`, and enforces a hard maximum retry count (no retry
storms). Every attempt's sanitised result is recorded under **Notifications**
(the full webhook URL is never stored there).
