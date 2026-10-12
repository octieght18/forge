package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/octieght18/forge/internal/store"
)

// rateGate is a single-process fixed window. It does not coordinate with other API processes.
type rateGate struct {
	limit int
	mu    sync.Mutex
	used  map[string]rateWindow
}

type rateWindow struct {
	start time.Time
	count int
}

func newRate(perSecond int) *rateGate {
	if perSecond < 1 {
		perSecond = 1
	}
	return &rateGate{limit: perSecond, used: map[string]rateWindow{}}
}

func (g *rateGate) allow(tenant string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	window := g.used[tenant]
	if window.start.IsZero() || now.Sub(window.start) >= time.Second {
		g.used[tenant] = rateWindow{start: now, count: 1}
		return true
	}
	if window.count >= g.limit {
		return false
	}
	window.count++
	g.used[tenant] = window
	return true
}

func (h *registration) allow(w http.ResponseWriter, r *http.Request, p store.Principal) bool {
	tenant := p.Tenant
	if tenant == "" {
		tenant = store.DemoTenant
	}
	if h.rate.allow(tenant, time.Now()) {
		return true
	}
	w.Header().Set("Retry-After", "1")
	WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "Retry later")
	return false
}
