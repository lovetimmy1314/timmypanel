// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package middleware

import (
	"sync"
	"time"
)

// LoginLimiter 是登录失败限流器：同一 key（IP 或用户名）连续失败达到上限后锁定一段时间。
// 数据只放在进程内存里 —— 个人站点重启不频繁，没必要为此引入 Redis。
type LoginLimiter struct {
	mu       sync.Mutex
	entries  map[string]*entry
	max      int
	window   time.Duration
	lockFor  time.Duration
	lastSwep time.Time
}

type entry struct {
	fails     int
	firstFail time.Time
	lockUntil time.Time
}

// NewLoginLimiter 构造限流器：window 内失败 max 次则锁 lockFor。
func NewLoginLimiter(max int, window, lockFor time.Duration) *LoginLimiter {
	return &LoginLimiter{
		entries: make(map[string]*entry),
		max:     max,
		window:  window,
		lockFor: lockFor,
	}
}

// Locked 返回该 key 是否处于锁定中，以及剩余时间。
func (l *LoginLimiter) Locked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked()
	e, ok := l.entries[key]
	if !ok {
		return false, 0
	}
	if time.Now().Before(e.lockUntil) {
		return true, time.Until(e.lockUntil)
	}
	return false, 0
}

// Fail 记一次失败，达到阈值则锁定。
func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.firstFail) > l.window {
		l.entries[key] = &entry{fails: 1, firstFail: now}
		return
	}
	e.fails++
	if e.fails >= l.max {
		e.lockUntil = now.Add(l.lockFor)
		e.fails = 0
		e.firstFail = now
	}
}

// Reset 登录成功后清掉该 key 的失败记录。
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// sweepLocked 顺带清理过期条目，避免 map 无限增长。调用方须持锁。
func (l *LoginLimiter) sweepLocked() {
	now := time.Now()
	if now.Sub(l.lastSwep) < time.Minute {
		return
	}
	l.lastSwep = now
	for k, e := range l.entries {
		if now.After(e.lockUntil) && now.Sub(e.firstFail) > l.window {
			delete(l.entries, k)
		}
	}
}
