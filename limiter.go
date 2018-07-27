package httpfiles

import (
	"sync"
	"sync/atomic"
	"time"
)

type remoteLimit struct {
	last          time.Time
	connectionSem chan struct{}
	rpsTicker     *time.Ticker
	rpsCalls      int64
	transferred   int64
}

type limiter struct {
	options Options
	lock    sync.RWMutex
	remotes map[string]*remoteLimit
}

func newLimiter(opts Options) *limiter {
	return &limiter{
		options: opts,
		remotes: make(map[string]*remoteLimit),
	}
}

func (l *limiter) Acquire(addr string) bool {
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

func (l *limiter) Release(addr string) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	limit, ok := l.remotes[addr]
	if !ok {
		return
	}

	release(limit)
}

func (l *limiter) Transferred(addr string, n int64) {
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
