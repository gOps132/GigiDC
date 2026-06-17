package provider

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestServiceAddCredentialSealsAndStoresMetadataOnly(t *testing.T) {
	ctx := context.Background()
	store := &fakeCredentialStore{}
	sealer := &fakeSecretSealer{
		sealed: SealedSecret{
			Ciphertext: []byte("sealed-secret"),
			Nonce:      []byte("nonce"),
			KeyID:      "key-id",
		},
	}
	service := NewService(store, sealer, DefaultRegistry())
	req := validAddCredentialRequest()

	got, err := service.AddCredential(ctx, req)
	if err != nil {
		t.Fatalf("AddCredential returned error: %v", err)
	}

	wantAAD := "owner_type=guild;guild_id=guild-id;user_id=;provider_id=openai;label=main"
	if !bytes.Equal(sealer.plaintext, []byte(req.RawSecret)) {
		t.Fatalf("sealed plaintext = %q, want raw secret", sealer.plaintext)
	}
	if string(sealer.aad) != wantAAD {
		t.Fatalf("AAD = %q, want %q", sealer.aad, wantAAD)
	}
	if store.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", store.upsertCalls)
	}
	if bytes.Contains(store.credentialInput.Ciphertext, []byte(req.RawSecret)) {
		t.Fatalf("stored ciphertext leaked plaintext: %q", store.credentialInput.Ciphertext)
	}
	if string(store.credentialInput.Ciphertext) != "sealed-secret" || string(store.credentialInput.Nonce) != "nonce" {
		t.Fatalf("stored sealed bytes = %q/%q, want sealed-secret/nonce", store.credentialInput.Ciphertext, store.credentialInput.Nonce)
	}
	wantFingerprint, err := Fingerprint(req.RawSecret)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if store.credentialInput.Fingerprint != wantFingerprint {
		t.Fatalf("fingerprint = %q, want %q", store.credentialInput.Fingerprint, wantFingerprint)
	}
	if got.ID != "credential-id" || got.Fingerprint != wantFingerprint {
		t.Fatalf("record = %+v, want stored metadata record", got)
	}
	if len(got.Ciphertext) != 0 || len(got.Nonce) != 0 {
		t.Fatalf("returned record leaked sealed bytes: %+v", got)
	}
}

func TestServiceAddCredentialTrimsRawSecretBeforeSeal(t *testing.T) {
	store := &fakeCredentialStore{}
	sealer := &fakeSecretSealer{}
	service := NewService(store, sealer, DefaultRegistry())
	req := validAddCredentialRequest()
	req.RawSecret = "  sk-live-test-secret  "

	if _, err := service.AddCredential(context.Background(), req); err != nil {
		t.Fatalf("AddCredential returned error: %v", err)
	}
	if string(sealer.plaintext) != "sk-live-test-secret" {
		t.Fatalf("sealed plaintext = %q, want trimmed secret", sealer.plaintext)
	}
}

func TestServiceAddCredentialValidatesDependenciesAndRequest(t *testing.T) {
	tests := []struct {
		name    string
		service Service
		req     AddCredentialRequest
		want    string
	}{
		{
			name:    "missing store",
			service: NewService(nil, &fakeSecretSealer{}, DefaultRegistry()),
			req:     validAddCredentialRequest(),
			want:    "credential store is required",
		},
		{
			name:    "missing sealer",
			service: NewService(&fakeCredentialStore{}, nil, DefaultRegistry()),
			req:     validAddCredentialRequest(),
			want:    "secret sealer is required",
		},
		{
			name:    "empty secret",
			service: NewService(&fakeCredentialStore{}, &fakeSecretSealer{}, DefaultRegistry()),
			req: func() AddCredentialRequest {
				req := validAddCredentialRequest()
				req.RawSecret = " \t "
				return req
			}(),
			want: "secret is required",
		},
		{
			name:    "unknown provider",
			service: NewService(&fakeCredentialStore{}, &fakeSecretSealer{}, DefaultRegistry()),
			req: func() AddCredentialRequest {
				req := validAddCredentialRequest()
				req.ProviderID = "unknown"
				return req
			}(),
			want: "unknown provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.service.AddCredential(context.Background(), tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestServiceListCredentialsStripsSealedBytes(t *testing.T) {
	store := &fakeCredentialStore{credentials: []CredentialRecord{
		{
			ID:          "credential-id",
			OwnerType:   OwnerGuild,
			GuildID:     "guild-id",
			ProviderID:  ProviderOpenAI,
			Label:       "main",
			Ciphertext:  []byte("sealed-secret"),
			Nonce:       []byte("nonce"),
			KeyID:       "key-id",
			Fingerprint: "fingerprint",
			Status:      CredentialStatusActive,
		},
	}}
	service := NewService(store, &fakeSecretSealer{}, DefaultRegistry())

	got, err := service.ListCredentials(context.Background(), Scope{OwnerType: OwnerGuild, GuildID: "guild-id"})
	if err != nil {
		t.Fatalf("ListCredentials returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "credential-id" {
		t.Fatalf("credentials = %+v, want one credential", got)
	}
	if len(got[0].Ciphertext) != 0 || len(got[0].Nonce) != 0 {
		t.Fatalf("credential leaked sealed bytes: %+v", got[0])
	}
}

func TestServiceSelectModelProfileRejectsUnsupportedProviderPurpose(t *testing.T) {
	store := &fakeCredentialStore{}
	service := NewService(store, &fakeSecretSealer{}, DefaultRegistry())
	req := validSelectModelRequest()
	req.ProviderID = ProviderAnthropic
	req.Purpose = PurposeEmbedding

	err := service.SelectModelProfile(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "provider does not support purpose") {
		t.Fatalf("error = %v, want provider purpose rejection", err)
	}
	if store.selectCalls != 0 {
		t.Fatalf("select calls = %d, want 0", store.selectCalls)
	}
}

func TestServiceSelectModelProfileDelegatesNormalizedInput(t *testing.T) {
	store := &fakeCredentialStore{}
	service := NewService(store, &fakeSecretSealer{}, DefaultRegistry())

	err := service.SelectModelProfile(context.Background(), validSelectModelRequest())
	if err != nil {
		t.Fatalf("SelectModelProfile returned error: %v", err)
	}
	if store.selectCalls != 1 {
		t.Fatalf("select calls = %d, want 1", store.selectCalls)
	}
	if store.profileInput.ModelID != "gpt-4o-mini" || store.profileInput.SelectedByUserID != "actor-id" {
		t.Fatalf("profile input = %+v, want normalized model profile input", store.profileInput)
	}
}

func TestServiceRevokeCredentialDelegatesActorAndScope(t *testing.T) {
	store := &fakeCredentialStore{}
	service := NewService(store, &fakeSecretSealer{}, DefaultRegistry())
	scope := Scope{OwnerType: OwnerGuild, GuildID: "guild-id"}

	err := service.RevokeCredential(context.Background(), scope, "main", "actor-id")
	if err != nil {
		t.Fatalf("RevokeCredential returned error: %v", err)
	}
	if store.revokeCalls != 1 {
		t.Fatalf("revoke calls = %d, want 1", store.revokeCalls)
	}
	if store.revokedScope != scope || store.revokedLabel != "main" || store.revokedActorID != "actor-id" {
		t.Fatalf("revoke args = %+v/%q/%q, want request args", store.revokedScope, store.revokedLabel, store.revokedActorID)
	}
}

func TestServiceRotateCredentialUsesAddCredentialBehavior(t *testing.T) {
	store := &fakeCredentialStore{}
	sealer := &fakeSecretSealer{
		sealed: SealedSecret{Ciphertext: []byte("rotated"), Nonce: []byte("nonce"), KeyID: "key-id"},
	}
	service := NewService(store, sealer, DefaultRegistry())

	got, err := service.RotateCredential(context.Background(), validAddCredentialRequest())
	if err != nil {
		t.Fatalf("RotateCredential returned error: %v", err)
	}
	if got.ID != "credential-id" || store.upsertCalls != 1 {
		t.Fatalf("record/calls = %+v/%d, want add credential behavior", got, store.upsertCalls)
	}
}

func TestServiceActiveModelProfileDelegates(t *testing.T) {
	store := &fakeCredentialStore{activeProfile: ModelProfile{
		ID:           "profile-id",
		OwnerType:    OwnerGuild,
		GuildID:      "guild-id",
		Purpose:      PurposeChat,
		CredentialID: "credential-id",
		ProviderID:   ProviderOpenAI,
		ModelID:      "gpt-4o-mini",
		Enabled:      true,
	}}
	service := NewService(store, &fakeSecretSealer{}, DefaultRegistry())

	got, err := service.ActiveModelProfile(context.Background(), Scope{OwnerType: OwnerGuild, GuildID: "guild-id"}, PurposeChat)
	if err != nil {
		t.Fatalf("ActiveModelProfile returned error: %v", err)
	}
	if got.ID != "profile-id" || store.activeCalls != 1 {
		t.Fatalf("profile/calls = %+v/%d, want delegated active profile", got, store.activeCalls)
	}
}

func validAddCredentialRequest() AddCredentialRequest {
	return AddCredentialRequest{
		Owner:      Scope{OwnerType: OwnerGuild, GuildID: "guild-id"},
		ProviderID: ProviderOpenAI,
		Label:      "main",
		RawSecret:  "sk-live-test-secret",
		ActorID:    "actor-id",
	}
}

func validSelectModelRequest() SelectModelRequest {
	return SelectModelRequest{
		Owner:        Scope{OwnerType: OwnerGuild, GuildID: "guild-id"},
		Purpose:      PurposeChat,
		CredentialID: "credential-id",
		ProviderID:   ProviderOpenAI,
		ModelID:      " gpt-4o-mini ",
		ParamsJSON:   `{"temperature":0.2}`,
		ActorID:      "actor-id",
	}
}

type fakeCredentialStore struct {
	credentialInput CredentialInput
	profileInput    ModelProfileInput
	credentials     []CredentialRecord
	activeProfile   ModelProfile
	revokedScope    Scope
	revokedLabel    string
	revokedActorID  string
	upsertCalls     int
	revokeCalls     int
	listCalls       int
	selectCalls     int
	activeCalls     int
}

func (s *fakeCredentialStore) UpsertCredential(_ context.Context, input CredentialInput) (CredentialRecord, error) {
	s.upsertCalls++
	s.credentialInput = input
	return CredentialRecord{
		ID:              "credential-id",
		OwnerType:       input.Owner.OwnerType,
		GuildID:         input.Owner.GuildID,
		UserID:          input.Owner.UserID,
		ProviderID:      input.ProviderID,
		Label:           input.Label,
		KeyID:           input.KeyID,
		Fingerprint:     input.Fingerprint,
		Status:          CredentialStatusActive,
		LastTestStatus:  TestStatusUntested,
		CreatedByUserID: input.CreatedByUserID,
		UpdatedByUserID: input.UpdatedByUserID,
	}, nil
}

func (s *fakeCredentialStore) RevokeCredential(_ context.Context, scope Scope, label string, actorID string) error {
	s.revokeCalls++
	s.revokedScope = scope
	s.revokedLabel = label
	s.revokedActorID = actorID
	return nil
}

func (s *fakeCredentialStore) ListCredentials(_ context.Context, scope Scope) ([]CredentialRecord, error) {
	s.listCalls++
	return append([]CredentialRecord(nil), s.credentials...), nil
}

func (s *fakeCredentialStore) SelectModelProfile(_ context.Context, input ModelProfileInput) error {
	s.selectCalls++
	s.profileInput = input
	return nil
}

func (s *fakeCredentialStore) ActiveModelProfile(_ context.Context, scope Scope, purpose Purpose) (ModelProfile, error) {
	s.activeCalls++
	return s.activeProfile, nil
}

type fakeSecretSealer struct {
	plaintext []byte
	aad       []byte
	sealed    SealedSecret
}

func (s *fakeSecretSealer) Seal(_ context.Context, plaintext, aad []byte) (SealedSecret, error) {
	s.plaintext = append([]byte(nil), plaintext...)
	s.aad = append([]byte(nil), aad...)
	if len(s.sealed.Ciphertext) == 0 {
		s.sealed = SealedSecret{Ciphertext: []byte("sealed-secret"), Nonce: []byte("nonce"), KeyID: "key-id"}
	}
	return s.sealed, nil
}

func (s *fakeSecretSealer) Open(_ context.Context, sealed SealedSecret, aad []byte) ([]byte, error) {
	return nil, nil
}
