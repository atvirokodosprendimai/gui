package dashboard

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	renderTempl(w, r, http.StatusOK, IndexPage(u.Email, u.Role))
}

func (a *App) handleAddServer(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		a.setFlash("admin role required")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	baseURL := strings.TrimSpace(r.URL.Query().Get("url"))
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	port := 8080
	if p := strings.TrimSpace(r.URL.Query().Get("port")); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	if name == "" || baseURL == "" {
		a.setFlash("name and url are required")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	id := nodeID(name, baseURL, port)
	n := node{ID: id, Name: name, URL: baseURL, Port: port, Token: token}
	if n.endpoint() == "" {
		a.setFlash("invalid server URL")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	a.mu.Lock()
	a.nodes[id] = n
	a.mu.Unlock()

	a.syncOnce()
	a.setFlash("server added: " + name)
	a.notifyReadModelChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		a.setFlash("admin role required")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusForbidden)
		return
	}
	id := chi.URLParam(r, "id")
	a.mu.Lock()
	delete(a.nodes, id)
	a.mu.Unlock()

	a.setFlash("server removed")
	a.notifyReadModelChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleParkDomain(w http.ResponseWriter, r *http.Request) {
	domain := normalizeFQDN(strings.TrimSpace(r.URL.Query().Get("domain")))
	ip := strings.TrimSpace(r.URL.Query().Get("ip"))
	rType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	zone := strings.TrimSpace(r.URL.Query().Get("zone"))
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	text := strings.TrimSpace(r.URL.Query().Get("text"))
	account := strings.TrimSpace(r.URL.Query().Get("account"))
	viewer := currentUser(r)
	ttl := 0
	if t := strings.TrimSpace(r.URL.Query().Get("ttl")); t != "" {
		if v, err := strconv.Atoi(t); err == nil {
			ttl = v
		}
	}

	if domain == "" {
		a.setFlash("domain is required")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	rec := normalizeRecord(dnsRecord{
		Name:   domain,
		Type:   rType,
		IP:     ip,
		Zone:   zone,
		Target: target,
		Text:   text,
		TTL:    ttl,
	})

	a.mu.Lock()
	a.dashboard[recordKey(rec)] = rec
	if viewer != nil && viewer.Role != roleAdmin {
		a.domainOwners[normalizeFQDN(rec.Name)] = viewer.Email
		_ = a.saveDomainOwner(normalizeFQDN(rec.Name), viewer.Email)
	} else if account != "" {
		a.domainOwners[normalizeFQDN(rec.Name)] = account
		_ = a.saveDomainOwner(normalizeFQDN(rec.Name), account)
	} else if _, ok := a.domainOwners[normalizeFQDN(rec.Name)]; !ok {
		a.domainOwners[normalizeFQDN(rec.Name)] = "unassigned"
		_ = a.saveDomainOwner(normalizeFQDN(rec.Name), "unassigned")
	}
	nodes := make([]node, 0, len(a.nodes))
	for _, n := range a.nodes {
		nodes = append(nodes, n)
	}
	a.mu.Unlock()

	failures := 0
	for _, n := range nodes {
		if err := a.addRecordToServer(n, rec); err != nil {
			failures++
			n.Online = false
			n.LastError = err.Error()
			n.LastSyncAt = time.Now().UTC()
			a.mu.Lock()
			a.nodes[n.ID] = n
			a.mu.Unlock()
		}
	}

	a.syncOnce()
	if failures > 0 {
		a.setFlash(fmt.Sprintf("parked %s with %d server errors", domain, failures))
	} else {
		a.setFlash("parked: " + domain)
	}
	a.notifyReadModelChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleTransferDomain(w http.ResponseWriter, r *http.Request) {
	domain := normalizeFQDN(strings.TrimSpace(r.URL.Query().Get("domain")))
	toAccount := strings.TrimSpace(r.URL.Query().Get("to_account"))
	viewer := currentUser(r)

	if viewer == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if domain == "" || toAccount == "" {
		a.setFlash("domain and target account are required")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if viewer.Role != roleAdmin {
		a.mu.RLock()
		owner := a.domainOwners[domain]
		a.mu.RUnlock()
		if owner != viewer.Email {
			a.setFlash("you can transfer only your own parked domains")
			a.notifyReadModelChanged()
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	if a.db != nil {
		if _, err := a.lookupUserByEmail(toAccount); err != nil {
			a.setFlash("target account does not exist")
			a.notifyReadModelChanged()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	a.mu.Lock()
	a.domainOwners[domain] = toAccount
	a.mu.Unlock()
	_ = a.saveDomainOwner(domain, toAccount)

	a.setFlash(fmt.Sprintf("transferred %s to account %s", domain, toAccount))
	a.notifyReadModelChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSyncNow(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		a.setFlash("admin role required")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusForbidden)
		return
	}
	a.syncOnce()
	a.setFlash("sync complete at " + time.Now().UTC().Format(time.RFC3339))
	a.notifyReadModelChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		a.setFlash("admin role required")
		a.notifyReadModelChanged()
		w.WriteHeader(http.StatusForbidden)
		return
	}
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	password := strings.TrimSpace(r.URL.Query().Get("password"))
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	if role == "" {
		role = roleUser
	}
	if err := a.CreateUser(email, password, role); err != nil {
		a.setFlash("failed to create user: " + err.Error())
	} else {
		a.setFlash("user created: " + strings.ToLower(email))
	}
	a.notifyReadModelChanged()
	w.WriteHeader(http.StatusNoContent)
}
