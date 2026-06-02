package storage

import (
	"os"
	"strings"
	"testing"
)

func TestInitialSchemaIncludesPermissionAndAuditTables(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000001_initial_schema.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"create table if not exists guilds",
		"create table if not exists role_capability_grants",
		"create table if not exists user_capability_grants",
		"create table if not exists audit_logs",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("schema missing %q", want)
		}
	}
	if strings.Contains(sql, "role_name") {
		t.Fatal("schema must map capabilities by role ID, not role name")
	}
}
