package telemetry

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"
	"go.opencensus.io/trace"
)

// redisHook is a hook for GoRedis client that adds instrumentation to all
// the commands executed by it.
func WrapRedis(client redis.UniversalClient) redis.UniversalClient {
	rs := &redisHook{}
	client.AddHook(rs)
	return client
}

type redisHook struct {
	addr string
}

func (rw *redisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	ctx, span := trace.StartSpan(ctx, operationName(cmd))

	args := cmd.Args()
	span.AddAttributes(trace.Int64Attribute("arg_count", int64(len(args))))
	if len(args) > 1 {
		switch cmd.Name() {
		case "evalsha":
			span.AddAttributes(trace.StringAttribute("sha", fmt.Sprintf("%v", cmd.Args()[1])))

		case "get", "set", "del", "zadd":
			span.AddAttributes(trace.StringAttribute("key", fmt.Sprintf("%v", cmd.Args()[1])))
		}
	}

	return ctx, nil
}

func (rw *redisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	trace.FromContext(ctx).End()
	return nil
}

func (rw *redisHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	ctx, _ = trace.StartSpan(ctx, operationName(cmds...))
	return ctx, nil
}

func (rw *redisHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	trace.FromContext(ctx).End()
	return nil
}

func operationName(cmds ...redis.Cmder) string {
	if len(cmds) == 1 {
		return "Redis_" + strings.ToUpper(cmds[0].Name())
	}

	operations := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		operations = append(operations, cmd.Name())
	}
	return "Redis_" + strings.ToUpper("pipeline:"+strings.Join(operations, ","))
}
