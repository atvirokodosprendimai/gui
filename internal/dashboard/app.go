package dashboard

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type App struct {
	mu           sync.RWMutex
	nodes        map[string]node
	dashboard    map[string]dnsRecord
	domainOwners map[string]string
	watchers     map[chan struct{}]struct{}
	client       *http.Client
	db           *gorm.DB
	syncInterval time.Duration
	flash        string
}

func New(client *http.Client, syncInterval time.Duration, dbs ...*gorm.DB) *App {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if syncInterval <= 0 {
		syncInterval = 15 * time.Second
	}

	var db *gorm.DB
	if len(dbs) > 0 {
		db = dbs[0]
	}

	app := &App{
		nodes:        make(map[string]node),
		dashboard:    make(map[string]dnsRecord),
		domainOwners: make(map[string]string),
		watchers:     make(map[chan struct{}]struct{}),
		client:       client,
		db:           db,
		syncInterval: syncInterval,
	}
	_ = app.loadDomainOwners()
	return app
}

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	r.Get("/login", a.handleLoginPage)
	r.Post("/auth/login", a.handleLogin)
	r.Get("/auth/logout", a.handleLogout)

	r.Group(func(pr chi.Router) {
		pr.Use(a.requireAuth)
		pr.Get("/", a.handleIndex)
		pr.Get("/ui/server/add", a.handleAddServer)
		pr.Get("/ui/server/delete/{id}", a.handleDeleteServer)
		pr.Get("/ui/domain/park", a.handleParkDomain)
		pr.Get("/ui/domain/transfer", a.handleTransferDomain)
		pr.Get("/ui/users/create", a.handleCreateUser)
		pr.Get("/ui/sync/now", a.handleSyncNow)
		pr.Get("/any/cqrs", a.handleCQRSStream)
	})

	return r
}

func (a *App) RunSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(a.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.syncOnce()
		}
	}
}

func (a *App) setFlash(msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.flash = msg
}

func (a *App) notifyReadModelChanged() {
	a.mu.RLock()
	watchers := make([]chan struct{}, 0, len(a.watchers))
	for ch := range a.watchers {
		watchers = append(watchers, ch)
	}
	a.mu.RUnlock()

	for _, ch := range watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (a *App) addWatcher() chan struct{} {
	ch := make(chan struct{}, 1)
	a.mu.Lock()
	a.watchers[ch] = struct{}{}
	a.mu.Unlock()
	return ch
}

func (a *App) removeWatcher(ch chan struct{}) {
	a.mu.Lock()
	delete(a.watchers, ch)
	a.mu.Unlock()
}

func (a *App) consumeFlash() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	msg := a.flash
	a.flash = ""
	return msg
}

func (a *App) overviewCounts() (nodeCount, onlineCount, recordCount int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	nodeCount = len(a.nodes)
	recordCount = len(a.dashboard)
	for _, n := range a.nodes {
		if n.Online {
			onlineCount++
		}
	}
	return nodeCount, onlineCount, recordCount
}

func (a *App) sortedNodes() []node {
	a.mu.RLock()
	defer a.mu.RUnlock()

	nodes := make([]node, 0, len(a.nodes))
	for _, n := range a.nodes {
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	return nodes
}

func (a *App) filteredRecordRows(filter string, viewer *User) []recordRow {
	filter = strings.ToLower(strings.TrimSpace(filter))

	a.mu.RLock()
	recs := make([]dnsRecord, 0, len(a.dashboard))
	owners := make(map[string]string, len(a.domainOwners))
	for _, rec := range a.dashboard {
		recs = append(recs, rec)
	}
	for domain, owner := range a.domainOwners {
		owners[domain] = owner
	}
	a.mu.RUnlock()

	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Name == recs[j].Name {
			if recs[i].Type == recs[j].Type {
				return recordValue(recs[i]) < recordValue(recs[j])
			}
			return recs[i].Type < recs[j].Type
		}
		return recs[i].Name < recs[j].Name
	})

	rows := make([]recordRow, 0, len(recs))
	for _, rec := range recs {
		owner := owners[normalizeFQDN(rec.Name)]
		if owner == "" {
			owner = "unassigned"
		}
		if viewer != nil && viewer.Role != roleAdmin && owner != viewer.Email {
			continue
		}
		line := strings.ToLower(rec.Name + " " + rec.Type + " " + recordValue(rec) + " " + rec.Zone + " " + owner)
		if filter == "" || strings.Contains(line, filter) {
			rows = append(rows, recordRow{Record: rec, Account: owner})
		}
	}

	return rows
}
