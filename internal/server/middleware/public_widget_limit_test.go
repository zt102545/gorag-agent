package middleware

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestPublicIPLimiterMinuteAndDailyLimits(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("redis is unavailable: %v", err)
	}
	limiter := NewPublicIPLimiter(client)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	for i := 0; i < 2; i++ {
		result, err := limiter.Allow(ctx, "minute-"+suffix, "192.0.2.1", 2, 100)
		if err != nil || !result.Allowed {
			t.Fatalf("minute request %d: result=%+v err=%v", i+1, result, err)
		}
	}
	result, err := limiter.Allow(ctx, "minute-"+suffix, "192.0.2.1", 2, 100)
	if err != nil || result.Allowed || result.DailyLimited {
		t.Fatalf("expected minute limit: result=%+v err=%v", result, err)
	}

	for i := 0; i < 3; i++ {
		result, err = limiter.Allow(ctx, "daily-"+suffix, "192.0.2.2", 100, 3)
		if err != nil || !result.Allowed {
			t.Fatalf("daily request %d: result=%+v err=%v", i+1, result, err)
		}
	}
	result, err = limiter.Allow(ctx, "daily-"+suffix, "192.0.2.2", 100, 3)
	if err != nil || result.Allowed || !result.DailyLimited {
		t.Fatalf("expected daily limit: result=%+v err=%v", result, err)
	}
}
