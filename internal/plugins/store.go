package plugins

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type catalogRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type catalogTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

type catalogRowQueryDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (catalogRows, error)
}

type catalogTxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (catalogTx, error)
}

type SQLCatalogStore struct {
	beginTx func(context.Context) (catalogTx, error)
	query   func(context.Context, string, ...any) (catalogRows, error)
	newID   func() string
}

func NewSQLCatalogStore(db any, newID func() string) SQLCatalogStore {
	store := SQLCatalogStore{newID: newID}
	if beginner, ok := db.(catalogTxBeginner); ok {
		store.beginTx = func(ctx context.Context) (catalogTx, error) {
			return beginner.BeginTx(ctx, nil)
		}
	} else if beginner, ok := db.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}); ok {
		store.beginTx = func(ctx context.Context) (catalogTx, error) {
			return beginner.BeginTx(ctx, nil)
		}
	}
	if queryDB, ok := db.(catalogRowQueryDB); ok {
		store.query = queryDB.QueryContext
	} else if queryDB, ok := db.(interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	}); ok {
		store.query = func(ctx context.Context, query string, args ...any) (catalogRows, error) {
			return queryDB.QueryContext(ctx, query, args...)
		}
	}
	return store
}

func (s SQLCatalogStore) UpsertApprovedManifest(ctx context.Context, manifest Manifest, actorID string) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if s.beginTx == nil {
		return fmt.Errorf("plugin catalog transaction is required")
	}
	if s.newID == nil {
		return fmt.Errorf("plugin version ID generator is required")
	}
	versionID := strings.TrimSpace(s.newID())
	if versionID == "" {
		return fmt.Errorf("plugin version ID is required")
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	manifestHash := sha256.Sum256(manifestJSON)

	tx, err := s.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin plugin catalog transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
insert into plugins (
  id,
  name,
  source,
  source_kind,
  discord_application_id,
  discord_bot_user_id,
  manifest_url,
  updated_at
)
values ($1, $2, $3, $4, $5, $6, $7, now())
on conflict (id) do update set
  name = excluded.name,
  source = excluded.source,
  source_kind = excluded.source_kind,
  discord_application_id = excluded.discord_application_id,
  discord_bot_user_id = excluded.discord_bot_user_id,
  manifest_url = excluded.manifest_url,
  updated_at = now()
`, strings.TrimSpace(manifest.ID), strings.TrimSpace(manifest.Name), strings.TrimSpace(manifest.Source), string(manifest.SourceKind), strings.TrimSpace(manifest.DiscordApplicationID), strings.TrimSpace(manifest.DiscordBotUserID), strings.TrimSpace(manifest.ManifestURL)); err != nil {
		return fmt.Errorf("upsert plugin: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
insert into plugin_versions (
  id,
  plugin_id,
  version,
  manifest,
  manifest_sha256,
  source_url,
  approved,
  approved_by_user_id
)
values ($1, $2, $3, $4, $5, $6, true, $7)
on conflict (plugin_id, version) do update set
  manifest = excluded.manifest,
  manifest_sha256 = excluded.manifest_sha256,
  source_url = excluded.source_url,
  approved = true,
  approved_by_user_id = excluded.approved_by_user_id
`, versionID, strings.TrimSpace(manifest.ID), strings.TrimSpace(manifest.Version), string(manifestJSON), hex.EncodeToString(manifestHash[:]), strings.TrimSpace(manifest.ManifestURL), strings.TrimSpace(actorID)); err != nil {
		return fmt.Errorf("upsert plugin version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit plugin catalog transaction: %w", err)
	}
	committed = true
	return nil
}

func (s SQLCatalogStore) ApprovedManifests(ctx context.Context) ([]Manifest, error) {
	if s.query == nil {
		return nil, fmt.Errorf("plugin catalog query database is required")
	}
	rows, err := s.query(ctx, `
select pv.manifest
from plugin_versions pv
join plugins p on p.id = pv.plugin_id
where pv.approved = true
order by p.name, pv.version
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanManifests(rows)
}

func (s SQLCatalogStore) EnableForGuild(ctx context.Context, guildID string, pluginID string, version string, actorID string) error {
	guildID = strings.TrimSpace(guildID)
	pluginID = strings.TrimSpace(pluginID)
	version = strings.TrimSpace(version)
	if guildID == "" {
		return fmt.Errorf("guild ID is required")
	}
	if pluginID == "" {
		return fmt.Errorf("plugin ID is required")
	}
	if version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if s.beginTx == nil {
		return fmt.Errorf("plugin catalog transaction is required")
	}
	if s.newID == nil {
		return fmt.Errorf("plugin install ID generator is required")
	}
	installID := strings.TrimSpace(s.newID())
	if installID == "" {
		return fmt.Errorf("plugin install ID is required")
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin plugin catalog transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
insert into guild_plugin_installs (
  id,
  guild_id,
  plugin_version_id,
  enabled,
  configured_by_user_id,
  enabled_by_user_id,
  updated_at
)
select $1, $2, pv.id, true, $3, $3, now()
from plugin_versions pv
where pv.plugin_id = $4
  and pv.version = $5
  and pv.approved = true
on conflict (guild_id, plugin_version_id) do update set
  enabled = true,
  configured_by_user_id = excluded.configured_by_user_id,
  enabled_by_user_id = excluded.enabled_by_user_id,
  updated_at = now()
`, installID, guildID, strings.TrimSpace(actorID), pluginID, version)
	if err != nil {
		return fmt.Errorf("enable guild plugin: %w", err)
	}
	if err := requireRowsAffected(result, "approved plugin version was not found"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit plugin catalog transaction: %w", err)
	}
	committed = true
	return nil
}

func (s SQLCatalogStore) DisableForGuild(ctx context.Context, guildID string, pluginID string, actorID string) error {
	guildID = strings.TrimSpace(guildID)
	pluginID = strings.TrimSpace(pluginID)
	if guildID == "" {
		return fmt.Errorf("guild ID is required")
	}
	if pluginID == "" {
		return fmt.Errorf("plugin ID is required")
	}
	if s.beginTx == nil {
		return fmt.Errorf("plugin catalog transaction is required")
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin plugin catalog transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
update guild_plugin_installs gpi
set
  enabled = false,
  configured_by_user_id = $3,
  updated_at = now()
from plugin_versions pv
where gpi.plugin_version_id = pv.id
  and gpi.guild_id = $1
  and pv.plugin_id = $2
  and gpi.enabled = true
`, guildID, pluginID, strings.TrimSpace(actorID))
	if err != nil {
		return fmt.Errorf("disable guild plugin: %w", err)
	}
	if err := requireRowsAffected(result, "enabled plugin was not found"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit plugin catalog transaction: %w", err)
	}
	committed = true
	return nil
}

func (s SQLCatalogStore) EnabledForGuild(ctx context.Context, guildID string) ([]Manifest, error) {
	if strings.TrimSpace(guildID) == "" {
		return nil, fmt.Errorf("guild ID is required")
	}
	if s.query == nil {
		return nil, fmt.Errorf("plugin catalog query database is required")
	}
	rows, err := s.query(ctx, `
select pv.manifest
from guild_plugin_installs gpi
join plugin_versions pv on pv.id = gpi.plugin_version_id
join plugins p on p.id = pv.plugin_id
where gpi.guild_id = $1
  and gpi.enabled = true
  and pv.approved = true
order by p.name, pv.version
`, strings.TrimSpace(guildID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanManifests(rows)
}

func scanManifests(rows catalogRows) ([]Manifest, error) {
	var manifests []Manifest
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan plugin manifest: %w", err)
		}
		var manifest Manifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, fmt.Errorf("decode stored plugin manifest: %w", err)
		}
		if err := manifest.Validate(); err != nil {
			return nil, fmt.Errorf("stored plugin manifest is invalid: %w", err)
		}
		manifests = append(manifests, manifest)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return manifests, nil
}

func requireRowsAffected(result sql.Result, emptyMessage string) error {
	if result == nil {
		return nil
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("%s", emptyMessage)
	}
	return nil
}
