package dashboard

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
)

type App struct {
	mu           sync.RWMutex
	nodes        map[string]node
	dashboard    map[string]dnsRecord
	domainOwners map[string]string
	watchers     map[chan struct{}]struct{}
	client       *http.Client
	syncInterval time.Duration
	flash        string
}

func New(client *http.Client, syncInterval time.Duration) *App {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if syncInterval <= 0 {
		syncInterval = 15 * time.Second
	}

	return &App{
		nodes:        make(map[string]node),
		dashboard:    make(map[string]dnsRecord),
		domainOwners: make(map[string]string),
		watchers:     make(map[chan struct{}]struct{}),
		client:       client,
		syncInterval: syncInterval,
	}
}

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", a.handleIndex)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	r.Get("/fragments/overview", a.handleOverviewFragment)
	r.Get("/fragments/servers", a.handleServersFragment)
	r.Get("/fragments/records", a.handleRecordsFragment)

	r.Get("/ui/server/add", a.handleAddServer)
	r.Get("/ui/server/delete/{id}", a.handleDeleteServer)
	r.Get("/ui/domain/park", a.handleParkDomain)
	r.Get("/ui/domain/transfer", a.handleTransferDomain)
	r.Get("/ui/sync/now", a.handleSyncNow)
	r.Get("/any/cqrs", a.handleCQRSStream)

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

func (a *App) writeCombinedFragments(w http.ResponseWriter, r *http.Request, filter string) {
	nodeCount, onlineCount, recordCount := a.overviewCounts()
	renderTempl(w, r, http.StatusOK, templ.Join(
		FlashFragment(a.consumeFlash()),
		OverviewFragment(nodeCount, onlineCount, recordCount),
		ServersFragment(a.sortedNodes()),
		RecordsFragment(a.filteredRecordRows(filter)),
	))
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

func (a *App) filteredRecordRows(filter string) []recordRow {
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
		line := strings.ToLower(rec.Name + " " + rec.Type + " " + recordValue(rec) + " " + rec.Zone + " " + owner)
		if filter == "" || strings.Contains(line, filter) {
			rows = append(rows, recordRow{Record: rec, Account: owner})
		}
	}

	return rows
}
