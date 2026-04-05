# Schedule

A **Schedule** is defined using crontab expression. Whenever the schedule is ready (i.e., the crontab qualifies the
current time), the **Scheduler** generates an **Execution Request** and publishes over the configured **Channel** for
the client that created the schedule (Refer [clients](clients.md) for details about clients, channels, etc.).

For example, consider the following Schedule (Illustrative format):

```json
{
  "id": "everysec-test",
  "tags": [
    "foo",
    "bar",
    "team:shield"
  ],
  "status": "ACTIVE",
  "crontab": "@every 1m",
  "version": 0,
  "category": "test",
  "payload": "Greetings!",
  "client_id": "dev-test",
  "created_at": "2009-11-11T00:00:00Z",
  "updated_at": "2009-11-11T00:00:00Z",
  "enqueue_count": 1,
  "last_enqueued_at": "2021-04-08T07:21:47Z"
}
```

If this was created on `2009-11-11T00:00:00Z`, Execution Requests must be generated at `2009-11-11T00:01:00Z`,
`2009-11-11T00:02:00Z`, `2009-11-11T00:03:00Z`, etc. The first Execution Request would be:

```json
{
  "schedule_id": "42f62d66bb1d",
  "enqueue_at": "2009-11-11T00:01:00Z",
  "version": 0,
  "manual": false
}
```

## Crontab Format

Clockwork supports standard UNIX-style crontab expressions (e.g., `5 0 * 8 *` to represent `At 00:05 in August.`). Clockwork
also supports some non-standard extensions to simplify usage:

* `@every 1s` - Execute every 1s
* `@at 1622635373,1622655373` - Execute at `1622635373` (i.e., `2021-06-02 17:32:53 +0530 IST`) and `1622655373` (
  i.e., `2021-06-02 23:06:13 +0530 IST` )

> Note: Refer [robfig/cron at godoc](https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format) for
> complete specification.
