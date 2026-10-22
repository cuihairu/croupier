---
title: Notification Channels
icon: notification
order: 23
category:
  - Operations Manual
tag:
  - Observability
  - Notifications
---

# Notification Channels

The unified outlet for platform events: **approval events, alert triggers, function call events**. The four channels have independent switches, and configuration takes effect hot (L3 override layer, see Config Layering).

## Channels and Configuration Keys

| Channel         | Configuration key (`PUT /api/v1/site/<key>`)                                                                                                                                                         | Description                                                                                                                                                            |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| In-app messages | `notification.inAppEnabled` (on by default)                                                                                                                                                          | Writes to the `messages` table + real-time SSE push, zero external dependencies                                                                                        |
| DingTalk bot    | `notification.dingtalkUrl` / `dingtalkSecret`                                                                                                                                                        | Bot webhook + signing                                                                                                                                                  |
| Custom webhook  | `notification.webhookUrl` / `webhookSecret`                                                                                                                                                          | POSTs a JSON event body; can integrate with self-hosted WeCom/Feishu apps                                                                                              |
| SMTP email      | `notification.emailEnabled` + `smtpHost` / `smtpPort` / `smtpUser` / `smtpPassword` / `smtpFrom` / `smtpEncryption` (none\|ssl\|starttls) / `smtpAuthType` (plain\|login) / `smtpInsecureSkipVerify` | The password is masked after being written (only "set + last 4 digits" is echoed); for the encryption and auth matrix see the "SMTP Email Service" card in the Ops tab |

How to configure (Dashboard "System Administration → Site Configuration", or directly via the API):

```bash
# Set the DingTalk bot
curl -X PUT https://<server>:18780/api/v1/site/notification.dingtalkUrl \
  -H "Authorization: Bearer ***" \
  -d '{"value":"https://oapi.dingtalk.com/robot/send?access_token=xxx"}'

# View the channel snapshot (with masking and source annotations)
curl -H "Authorization: Bearer ***" https://<server>:18780/api/v1/site/notification

# Clear the override = fall back to the config file
curl -X DELETE -H "Authorization: Bearer ***" \
  https://<server>:18780/api/v1/site/notification.dingtalkUrl
```

Key/value semantics: value is a JSON value (string/boolean); deleting an override falls back to the YAML default — **a mistake can be rolled back without a restart**.

## Events and Bindings

| Event                                                               | Trigger source         |
| ------------------------------------------------------------------- | ---------------------- |
| Approval flow transitions (pending/approved/rejected)               | Approval Center        |
| Alert fired (`alert.fired`)                                         | Alert rule engine      |
| Certificate expiry (`certificate_expiring` / `certificate_expired`) | Certificate Monitoring |
| Function call events                                                | Call path              |

On the ops side there is also event → channel routing (`GET/PUT /api/v1/ops/notifications`, stored in `ops_state.json`): `{channels:[{id,type,url,secret}], rules:[{event, channels, thresholdDays}]}` — which channels receive which events can be picked per event (e.g. certificate expiry only to DingTalk, not email).

## Reliability Semantics

- Dispatch failures are **only logged and never block business** — notifications are best-effort; alert persistence is the source of truth
- Webhook receivers must be idempotent (duplicate pushes may arrive)
- Secret-class values (SMTP password/signing secret) are always read masked, and audit logs are masked in sync

## Troubleshooting

| Symptom                         | Checks                                                                                                                               |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| In-app works, DingTalk does not | `GET /api/v1/site/notification` to confirm the URL is set and the channel is not excluded by ops routing                             |
| Webhook not received            | Connectivity from the Server's egress to the target address; look for the `notify` keyword in Server logs                            |
| All emails fail                 | SMTP account/port (465 SSL vs 587 STARTTLS depending on the provider); whether the password was written back as a masked placeholder |
| Config change not taking effect | L3 overrides are immediate; confirm you changed the `notification.*` keys, not the YAML (YAML requires a restart)                    |
