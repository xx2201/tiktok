package server

import (
	"crypto/sha256"
	"fmt"
	"github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"golang.org/x/time/rate"
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	limiter *rate.Limiter
	last    time.Time
}

func rateLimit(limit float64, burst int) khttp.FilterFunc {
	var mu sync.Mutex
	buckets := make(map[string]*bucket)
	lastCleanup := time.Now()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				next.ServeHTTP(w, r)
				return
			}
			key := r.URL.Query().Get("token")
			if key == "" {
				key, _, _ = net.SplitHostPort(r.RemoteAddr)
			}
			key = fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
			now := time.Now()
			mu.Lock()
			if now.Sub(lastCleanup) > time.Minute {
				for k, b := range buckets {
					if now.Sub(b.last) > 10*time.Minute {
						delete(buckets, k)
					}
				}
				lastCleanup = now
			}
			b := buckets[key]
			if b == nil {
				b = &bucket{limiter: rate.NewLimiter(rate.Limit(limit), burst)}
				buckets[key] = b
			}
			b.last = now
			allowed := b.limiter.Allow()
			mu.Unlock()
			if !allowed {
				errorEncoder(w, r, errors.New(429, "RATE_LIMITED", "请求过于频繁"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
