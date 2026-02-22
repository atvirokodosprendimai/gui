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
	watchers     map[chan uiUpdate]struct{}
	client       *http.Client
	db           *gorm.DB
	syncInterval time.Duration
	flashBySess  map[string]string
	initErr      error
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
		watchers:     make(map[chan uiUpdate]struct{}),
		client:       client,
		db:           db,
		syncInterval: syncInterval,
		flashBySess:  make(map[string]string),
	}
	if err := app.loadDomainOwners(); err != nil {
		app.initErr = err
	}
	return app
}

func (a *App) InitError() error {
	return a.initErr
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
		pr.Post("/ui/server/add", a.handleAddServer)
		pr.Post("/ui/server/delete/{id}", a.handleDeleteServer)
		pr.Post("/ui/domain/park", a.handleParkDomain)
		pr.Post("/ui/domain/transfer", a.handleTransferDomain)
		pr.Post("/ui/users/create", a.handleCreateUser)
		pr.Post("/ui/sync/now", a.handleSyncNow)
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

func sessionTokenFromRequest(r *http.Request) string {
	c, err := r.Cookie("session_token")
	if err != nil {
		return ""
	}
	return c.Value
}

func (a *App) setFlash(r *http.Request, msg string) {
	key := sessionTokenFromRequest(r)
	if key == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.flashBySess[key] = msg
}

type uiUpdate struct {
	Subject string `json:"subject,omitempty"`
	Element string `json:"el"`
}

const subjectFEUpdate = "fe.update"

func (a *App) notifyElementsChanged(elements ...string) {
	a.mu.RLock()
	watchers := make([]chan uiUpdate, 0, len(a.watchers))
	for ch := range a.watchers {
		watchers = append(watchers, ch)
	}
	a.mu.RUnlock()

	for _, el := range elements {
		upd := uiUpdate{Subject: subjectFEUpdate, Element: el}
		for _, ch := range watchers {
			select {
			case ch <- upd:
			default:
			}
		}
	}
}

func (a *App) notifyReadModelChanged() {
	a.notifyElementsChanged("flash", "overview", "servers", "records", "users")
}

func (a *App) addWatcher() chan uiUpdate {
	ch := make(chan uiUpdate, 8)
	a.mu.Lock()
	a.watchers[ch] = struct{}{}
	a.mu.Unlock()
	return ch
}

func (a *App) removeWatcher(ch chan uiUpdate) {
	a.mu.Lock()
	delete(a.watchers, ch)
	a.mu.Unlock()
}

func (a *App) consumeFlash(r *http.Request) string {
	key := sessionTokenFromRequest(r)
	if key == "" {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	msg := a.flashBySess[key]
	delete(a.flashBySess, key)
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

func (a *App) sortedUsers() []User {
	if a.db == nil {
		return nil
	}
	users := make([]User, 0)
	if err := a.db.Select("id", "email", "role", "created_at").Order("email asc").Find(&users).Error; err != nil {
		return nil
	}
	return users
}
