package dashboard

import (
	"fmt"
	"net/http"
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
		a.setFlash(r, "admin role required")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var sig addServerSignals
	if err := readDatastarSignals(r, &sig); err != nil {
		a.setFlash(r, "invalid request payload")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	name := trim(sig.ServerName)
	baseURL := trim(sig.ServerURL)
	token := trim(sig.ServerToken)
	port := parseIntOrDefault(sig.ServerPort, 8080)

	if name == "" || baseURL == "" {
		a.setFlash(r, "name and url are required")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	id := nodeID(name, baseURL, port)
	n := node{ID: id, Name: name, URL: baseURL, Port: port, Token: token}
	if n.endpoint() == "" {
		a.setFlash(r, "invalid server URL")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	a.mu.Lock()
	a.nodes[id] = n
	a.mu.Unlock()

	a.syncOnce()
	a.setFlash(r, "server added: "+name)
	a.notifySessionElements(r, "flash")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		a.setFlash(r, "admin role required")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	id := chi.URLParam(r, "id")
	a.mu.Lock()
	delete(a.nodes, id)
	a.mu.Unlock()

	a.setFlash(r, "server removed")
	a.notifySessionElements(r, "flash")
	a.notifyElementsChanged("servers", "overview")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleParkDomain(w http.ResponseWriter, r *http.Request) {
	var sig parkDomainSignals
	if err := readDatastarSignals(r, &sig); err != nil {
		a.setFlash(r, "invalid request payload")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	domain := normalizeFQDN(trim(sig.Domain))
	ip := trim(sig.RecordIP)
	rType := strings.ToUpper(trim(sig.RecordType))
	zone := trim(sig.RecordZone)
	target := trim(sig.RecordTarget)
	text := trim(sig.RecordText)
	account := trim(sig.DomainAccount)
	viewer := currentUser(r)
	ttl := parseIntOrDefault(sig.RecordTTL, 0)

	if domain == "" {
		a.setFlash(r, "domain is required")
		a.notifySessionElements(r, "flash")
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
		if err := a.saveDomainOwner(normalizeFQDN(rec.Name), viewer.Email); err != nil {
			a.mu.Unlock()
			a.setFlash(r, "failed to save domain owner")
			a.notifySessionElements(r, "flash")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if account != "" {
		a.domainOwners[normalizeFQDN(rec.Name)] = account
		if err := a.saveDomainOwner(normalizeFQDN(rec.Name), account); err != nil {
			a.mu.Unlock()
			a.setFlash(r, "failed to save domain owner")
			a.notifySessionElements(r, "flash")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if _, ok := a.domainOwners[normalizeFQDN(rec.Name)]; !ok {
		a.domainOwners[normalizeFQDN(rec.Name)] = "unassigned"
		if err := a.saveDomainOwner(normalizeFQDN(rec.Name), "unassigned"); err != nil {
			a.mu.Unlock()
			a.setFlash(r, "failed to save domain owner")
			a.notifySessionElements(r, "flash")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
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
		a.setFlash(r, fmt.Sprintf("parked %s with %d server errors", domain, failures))
	} else {
		a.setFlash(r, "parked: "+domain)
	}
	a.notifySessionElements(r, "flash")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleTransferDomain(w http.ResponseWriter, r *http.Request) {
	var sig transferDomainSignals
	if err := readDatastarSignals(r, &sig); err != nil {
		a.setFlash(r, "invalid request payload")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	domain := normalizeFQDN(trim(sig.TransferDomain))
	toAccount := trim(sig.TransferToAccount)
	viewer := currentUser(r)

	if viewer == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if domain == "" || toAccount == "" {
		a.setFlash(r, "domain and target account are required")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if viewer.Role != roleAdmin {
		a.mu.RLock()
		owner := a.domainOwners[domain]
		a.mu.RUnlock()
		if owner != viewer.Email {
			a.setFlash(r, "you can transfer only your own parked domains")
			a.notifySessionElements(r, "flash")
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	if a.db != nil {
		if _, err := a.lookupUserByEmail(toAccount); err != nil {
			a.setFlash(r, "target account does not exist")
			a.notifySessionElements(r, "flash")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	a.mu.Lock()
	a.domainOwners[domain] = toAccount
	a.mu.Unlock()
	if err := a.saveDomainOwner(domain, toAccount); err != nil {
		a.setFlash(r, "failed to persist domain transfer")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	a.setFlash(r, fmt.Sprintf("transferred %s to account %s", domain, toAccount))
	a.notifySessionElements(r, "flash")
	a.notifyElementsChanged("records")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSyncNow(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		a.setFlash(r, "admin role required")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	a.syncOnce()
	a.setFlash(r, "sync complete at "+time.Now().UTC().Format(time.RFC3339))
	a.notifySessionElements(r, "flash")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		a.setFlash(r, "admin role required")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var sig createUserSignals
	if err := readDatastarSignals(r, &sig); err != nil {
		a.setFlash(r, "invalid request payload")
		a.notifySessionElements(r, "flash")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	email := trim(sig.NewUserEmail)
	password := trim(sig.NewUserPassword)
	role := trim(sig.NewUserRole)
	if role == "" {
		role = roleUser
	}
	if err := a.CreateUser(email, password, role); err != nil {
		a.setFlash(r, "failed to create user: "+err.Error())
	} else {
		a.setFlash(r, "user created: "+strings.ToLower(email))
	}
	a.notifySessionElements(r, "flash")
	a.notifyElementsChanged("users")
	w.WriteHeader(http.StatusNoContent)
}
