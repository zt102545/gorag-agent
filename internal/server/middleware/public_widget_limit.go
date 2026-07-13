package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const publicLimitScript = `
local minute_count = redis.call('INCR', KEYS[1])
if minute_count == 1 then redis.call('EXPIRE', KEYS[1], ARGV[3]) end
if tonumber(ARGV[1]) > 0 and minute_count > tonumber(ARGV[1]) then
  return {1, minute_count, 0}
end
local daily_count = redis.call('INCR', KEYS[2])
if daily_count == 1 then redis.call('EXPIRE', KEYS[2], ARGV[4]) end
if tonumber(ARGV[2]) > 0 and daily_count > tonumber(ARGV[2]) then
  return {2, minute_count, daily_count}
end
return {0, minute_count, daily_count}
`

type PublicIPLimiter struct {
	redis redis.UniversalClient
}

type PublicLimitResult struct {
	Allowed      bool
	DailyLimited bool
	MinuteCount  int64
	DailyCount   int64
	RetryAfter   int
}

func NewPublicIPLimiter(client redis.UniversalClient) *PublicIPLimiter {
	return &PublicIPLimiter{redis: client}
}

func (l *PublicIPLimiter) Allow(ctx context.Context, channelID, ip string, ratePerMinute, dailyQuota int) (PublicLimitResult, error) {
	if l == nil || l.redis == nil {
		return PublicLimitResult{Allowed: true}, nil
	}
	if ratePerMinute <= 0 || dailyQuota <= 0 {
		return PublicLimitResult{}, fmt.Errorf("invalid public widget limit configuration")
	}
	now := time.Now().UTC()
	scope := fmt.Sprintf("%x", sha256.Sum256([]byte(channelID+"\x00"+ip)))
	minuteKey := "cs:public-widget:minute:" + scope + ":" + now.Format("200601021504")
	dailyKey := "cs:public-widget:day:" + scope + ":" + now.Format("20060102")
	values, err := l.redis.Eval(ctx, publicLimitScript, []string{minuteKey, dailyKey}, ratePerMinute, dailyQuota, 120, 172800).Slice()
	if err != nil {
		return PublicLimitResult{}, err
	}
	if len(values) != 3 {
		return PublicLimitResult{}, fmt.Errorf("unexpected public widget limiter result")
	}
	code, err := redisInt64(values[0])
	if err != nil {
		return PublicLimitResult{}, err
	}
	minuteCount, err := redisInt64(values[1])
	if err != nil {
		return PublicLimitResult{}, err
	}
	dailyCount, err := redisInt64(values[2])
	if err != nil {
		return PublicLimitResult{}, err
	}
	result := PublicLimitResult{Allowed: code == 0, DailyLimited: code == 2, MinuteCount: minuteCount, DailyCount: dailyCount, RetryAfter: 60}
	if code == 2 {
		result.RetryAfter = int(time.Until(now.Truncate(24 * time.Hour).Add(24 * time.Hour)).Seconds())
		if result.RetryAfter < 1 {
			result.RetryAfter = 1
		}
	}
	return result, nil
}

func redisInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected redis integer type %T", value)
	}
}
