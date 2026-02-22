package dashboard

import "time"

func (a *App) syncOnce() {
	a.mu.RLock()
	nodes := make([]node, 0, len(a.nodes))
	for _, n := range a.nodes {
		nodes = append(nodes, n)
	}

	union := make(map[string]dnsRecord, len(a.dashboard))
	for k, r := range a.dashboard {
		union[k] = normalizeRecord(r)
	}
	a.mu.RUnlock()

	serverViews := make(map[string]map[string]dnsRecord, len(nodes))

	for _, n := range nodes {
		recs, err := a.fetchRecords(n)
		if err != nil {
			n.LastError = err.Error()
			n.Online = false
			n.LastSyncAt = time.Now().UTC()
			a.mu.Lock()
			a.nodes[n.ID] = n
			a.mu.Unlock()
			continue
		}

		n.Online = true
		n.LastError = ""
		n.LastSyncAt = time.Now().UTC()
		n.RecordCount = len(recs)

		serverSet := make(map[string]dnsRecord, len(recs))
		for _, rec := range recs {
			normalized := normalizeRecord(rec)
			key := recordKey(normalized)
			serverSet[key] = normalized
			if _, ok := union[key]; !ok {
				union[key] = normalized
			}
		}

		serverViews[n.ID] = serverSet
		a.mu.Lock()
		a.nodes[n.ID] = n
		a.mu.Unlock()
	}

	for _, n := range nodes {
		serverSet, ok := serverViews[n.ID]
		if !ok {
			continue
		}

		for key, rec := range union {
			if _, exists := serverSet[key]; exists {
				continue
			}

			if err := a.addRecordToServer(n, rec); err != nil {
				n.Online = false
				n.LastError = err.Error()
				n.LastSyncAt = time.Now().UTC()
				a.mu.Lock()
				a.nodes[n.ID] = n
				a.mu.Unlock()
				break
			}
		}
	}

	a.mu.Lock()
	a.dashboard = union
	a.mu.Unlock()
	a.notifyReadModelChanged()
}
