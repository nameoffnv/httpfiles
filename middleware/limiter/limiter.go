package limiter

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Limiter struct {
	options Options
	lock    sync.RWMutex
	remotes map[string]*remoteLimit
}

type remoteLimit struct {
	last          time.Time
	connectionSem chan struct{}
	rpsTicker     *time.Ticker
	rpsCalls      int64
	transferred   int64
}

type counterResponseWriter struct {
	http.ResponseWriter
	bytesTransferred int64
}

func (c *counterResponseWriter) Write(b []byte) (int, error) {
	c.bytesTransferred += int64(len(b))
	return c.ResponseWriter.Write(b)
}

func New(opts Options) *Limiter {
	opts.Setup()

	limiter := &Limiter{
		options: opts,
		remotes: make(map[string]*remoteLimit),
	}

	return limiter
}

func (l *Limiter) LimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		crw := &counterResponseWriter{ResponseWriter: rw}
		remoteAddr := strings.Split(req.RemoteAddr, ":")[0]

		if l.acquire(remoteAddr) {
			next.ServeHTTP(rw, req)
			l.release(remoteAddr)
			l.transferred(remoteAddr, crw.bytesTransferred)
		} else {
			log.Printf("remote: %s - too many requests", remoteAddr)
			http.Error(rw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		}
	})
}

func (l *Limiter) acquire(addr string) bool {
	l.lock.RLock()

	limit, ok := l.remotes[addr]
	if !ok {
		l.lock.RUnlock()

		l.lock.Lock()
		l.remotes[addr] = &remoteLimit{
			last:          time.Now(),
			connectionSem: make(chan struct{}, l.options.MaxConnectionPerIP),
			rpsTicker:     time.NewTicker(1 * time.Second),
		}
		l.lock.Unlock()

		return acquire(l.remotes[addr], l.options)
	}

	l.lock.RUnlock()
	return acquire(limit, l.options)
}

func (l *Limiter) release(addr string) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	limit, ok := l.remotes[addr]
	if !ok {
		return
	}

	release(limit)
}

func (l *Limiter) transferred(addr string, n int64) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	limit, ok := l.remotes[addr]
	if !ok {
		return
	}

	atomic.AddInt64(&limit.transferred, n)
}

func acquire(limit *remoteLimit, limitOptions Options) bool {
	limit.last = time.Now()

	select {
	case <-limit.rpsTicker.C:
		atomic.StoreInt64(&limit.rpsCalls, 0)
	default:
	}

	calls := atomic.AddInt64(&limit.rpsCalls, 1)
	if calls > int64(limitOptions.MaxRequestPerSecond) {
		return false
	}

	if atomic.LoadInt64(&limit.transferred) > limitOptions.MaxBytesPerIP {
		return false
	}

	select {
	case limit.connectionSem <- struct{}{}:
	default:
		return false
	}

	return true
}

func release(limit *remoteLimit) {
	<-limit.connectionSem
}
