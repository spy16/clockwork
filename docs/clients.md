# Clients

A **Client** represents an entity that is integrating with the Clockwork system. Each client needs to be registered using
client management API.

Each client is registered with a _[channel](#channel)_. All the Schedules created by this client will use this channel
to publish the Execution Requests.

A sample Client definition is shown below:

```json
{
  "id": "dev-test",
  "secret": "xXbCdWxW0AYiUebo",
  "created_at": "2021-04-08T14:18:10.828953035+07:00",
  "updated_at": "2021-04-08T14:18:10.828953035+07:00",
  "channel_type": "kafka",
  "channel_name": "clockwork-events"
}
```

> Note: A client can be registered using the client management REST API (i.e., by doing a `POST /v1/clients` with
> client definition in body. Refer <http://clockwork.host/swagger/> for the exact semantics)

## Channel

Channel is used by the Clockwork to send execution requests. Currently, following channel types are supported:

1. `kafka` - Publishes the execution request to a Kafka topic defined using `channel_name` field in Client definition. 
   (Note that the Kafka cluster is configured at Clockwork service level)
2. `log` - Just logs the schedule details everytime an execution is due (Mostly used for debugging).
