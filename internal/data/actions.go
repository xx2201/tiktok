package data

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"github.com/go-kratos/kratos/v2/log"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"sync"
	"time"
)

type Actions struct {
	kind       string
	redis      *redis.Client
	connection *amqp.Connection
	publisher  *amqp.Channel
	consumer   *amqp.Channel
	returns    <-chan amqp.Return
	db         *Data
	interval   time.Duration
	log        *log.Helper
	mu         sync.Mutex
	lifecycle  sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}
}

func NewActions(ctx context.Context, kind string, r conf.Redis, mq conf.RabbitMQ, db *Data, logger log.Logger) (*Actions, error) {
	if kind != "favorite" && kind != "relation" {
		return nil, biz.ErrInvalid
	}
	a := &Actions{kind: kind, db: db, interval: mq.SyncInterval, log: log.NewHelper(logger), done: make(chan struct{})}
	a.redis = redis.NewClient(&redis.Options{Addr: r.Address, Password: r.Password, DB: r.DB})
	if err := a.redis.Ping(ctx).Err(); err != nil {
		a.redis.Close()
		return nil, err
	}
	connection, err := amqp.Dial(mq.URL)
	if err != nil {
		a.redis.Close()
		return nil, err
	}
	a.connection = connection
	a.publisher, err = connection.Channel()
	if err != nil {
		a.Close()
		return nil, err
	}
	if _, err = a.publisher.QueueDeclare(kind, true, false, false, false, nil); err != nil {
		a.Close()
		return nil, err
	}
	if err := a.publisher.Confirm(false); err != nil {
		a.Close()
		return nil, err
	}
	a.returns = a.publisher.NotifyReturn(make(chan amqp.Return, 1))
	a.consumer, err = connection.Channel()
	if err != nil {
		a.Close()
		return nil, err
	}
	if err := a.consumer.Qos(1, 0, false); err != nil {
		a.Close()
		return nil, err
	}
	return a, nil
}
func (a *Actions) Publish(ctx context.Context, action *biz.Action) error {
	if action.Kind != a.kind {
		return biz.ErrInvalid
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// 统一递增序号跨进程排序；Redis 丢失序号时显式依赖其持久化恢复。
	sequence, err := a.redis.Incr(ctx, "tiktok:action:sequence").Result()
	if err != nil {
		return err
	}
	action.CreatedAt = sequence
	data, err := json.Marshal(action)
	if err != nil {
		return err
	}
	confirmation, err := a.publisher.PublishWithDeferredConfirmWithContext(ctx, "", a.kind, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: data})
	if err != nil {
		return err
	}
	confirmed, err := confirmation.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("RabbitMQ rejected action")
	}
	select {
	case returned := <-a.returns:
		return fmt.Errorf("RabbitMQ returned action: %s", returned.ReplyText)
	default:
	}
	return nil
}

var cacheAction = redis.NewScript(`
local previous = redis.call('GET', KEYS[1])
if previous then
  local decoded = cjson.decode(previous)
  if decoded.created_at >= tonumber(ARGV[2]) then return 0 end
end
redis.call('SET', KEYS[1], ARGV[1])
redis.call('SET', KEYS[2], ARGV[1], 'EX', 6)
return 1
`)
var removeApplied = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0
`)

func (a *Actions) Cache(ctx context.Context, action *biz.Action) error {
	if action.Kind != a.kind || action.UserID <= 0 || action.TargetID <= 0 || action.CreatedAt <= 0 || (action.Type != 1 && action.Type != 2) {
		return biz.ErrInvalid
	}
	data, err := json.Marshal(action)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("tiktok:%s:%d:%d:w", a.kind, action.UserID, action.TargetID)
	return cacheAction.Run(ctx, a.redis, []string{key, key[:len(key)-1] + "r"}, string(data), action.CreatedAt).Err()
}
func (a *Actions) Sync(ctx context.Context) error {
	var cursor uint64
	for {
		keys, next, err := a.redis.Scan(ctx, cursor, "tiktok:"+a.kind+":*:w", 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			value, err := a.redis.Get(ctx, key).Result()
			if err == redis.Nil {
				continue
			}
			if err != nil {
				return err
			}
			var action biz.Action
			if err := json.Unmarshal([]byte(value), &action); err != nil {
				return err
			}
			if err := a.db.ApplyAction(ctx, &action); err != nil {
				return err
			}
			// 只有成功落库且值仍是这一版时才删除，失败或新动作都保留。
			if err := removeApplied.Run(ctx, a.redis, []string{key}, value).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
func (a *Actions) Start(ctx context.Context) error {
	a.lifecycle.Lock()
	ctx, a.cancel = context.WithCancel(ctx)
	a.lifecycle.Unlock()
	defer close(a.done)
	deliveries, err := a.consumer.Consume(a.kind, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("%s RabbitMQ consumer stopped", a.kind)
			}
			var action biz.Action
			if err := json.Unmarshal(delivery.Body, &action); err != nil {
				a.log.Errorf("invalid action rejected: %v", err)
				if err := delivery.Reject(false); err != nil {
					return err
				}
				continue
			}
			if err := a.Cache(ctx, &action); err != nil {
				if nackErr := delivery.Nack(false, true); nackErr != nil {
					return nackErr
				}
				return fmt.Errorf("cache action: %w", err)
			}
			if err := delivery.Ack(false); err != nil {
				return err
			}
		case <-ticker.C:
			if err := a.Sync(ctx); err != nil {
				a.log.Errorf("%s synchronization failed; pending state retained: %v", a.kind, err)
			}
		}
	}
}
func (a *Actions) Stop(ctx context.Context) error {
	a.lifecycle.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.lifecycle.Unlock()
	select {
	case <-a.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (a *Actions) Close() error {
	if a.connection != nil {
		_ = a.connection.Close()
	}
	if a.redis != nil {
		return a.redis.Close()
	}
	return nil
}
