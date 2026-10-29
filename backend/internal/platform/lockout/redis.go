package lockout

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a Limiter shared by every API instance.
type Redis struct {
	c      *redis.Client
	p      Policy
	prefix string
}

// NewRedis connects to url (redis://host:port/db) and pings it.
func NewRedis(ctx context.Context, url string, p Policy) (*Redis, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.Ping(pctx).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &Redis{c: c, p: p, prefix: "psaas:lockout:"}, nil
}

func (r *Redis) Locked(ctx context.Context, key string) (bool, time.Duration, error) {
	d, err := r.c.PTTL(ctx, r.prefix+"lock:"+key).Result()
	if err != nil {
		return false, 0, err
	}
	return d > 0, max(d, 0), nil
}

var failScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
if n >= tonumber(ARGV[2]) then
  redis.call('SET', KEYS[2], '1', 'PX', ARGV[3])
  redis.call('DEL', KEYS[1])
  return 1
end
return 0`)

func (r *Redis) Fail(ctx context.Context, key string) (bool, error) {
	n, err := failScript.Run(ctx, r.c, []string{r.prefix + "fail:" + key, r.prefix + "lock:" + key},
		r.p.Window.Milliseconds(), r.p.MaxFailures, r.p.LockFor.Milliseconds()).Int()
	return n == 1, err
}

func (r *Redis) Reset(ctx context.Context, key string) error {
	return r.c.Del(ctx, r.prefix+"fail:"+key, r.prefix+"lock:"+key).Err()
}

// Close releases the connection pool.
func (r *Redis) Close() error { return r.c.Close() }
