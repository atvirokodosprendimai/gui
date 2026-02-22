package dashboard

type domainOwnerRow struct {
	Domain     string `gorm:"column:domain;primaryKey"`
	OwnerEmail string `gorm:"column:owner_email"`
}

func (domainOwnerRow) TableName() string {
	return "domain_owners"
}

func (a *App) loadDomainOwners() error {
	if a.db == nil {
		return nil
	}
	var rows []domainOwnerRow
	if err := a.db.Find(&rows).Error; err != nil {
		return err
	}
	a.mu.Lock()
	for _, row := range rows {
		a.domainOwners[normalizeFQDN(row.Domain)] = row.OwnerEmail
	}
	a.mu.Unlock()
	return nil
}

func (a *App) saveDomainOwner(domain, ownerEmail string) error {
	if a.db == nil {
		return nil
	}
	domain = normalizeFQDN(domain)
	if domain == "" {
		return nil
	}
	row := domainOwnerRow{Domain: domain, OwnerEmail: ownerEmail}
	return a.db.Save(&row).Error
}
