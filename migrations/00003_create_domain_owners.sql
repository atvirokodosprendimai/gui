-- +goose Up
CREATE TABLE IF NOT EXISTS domain_owners (
  domain TEXT PRIMARY KEY,
  owner_email TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_domain_owners_owner_email ON domain_owners(owner_email);

-- +goose Down
DROP TABLE IF EXISTS domain_owners;
