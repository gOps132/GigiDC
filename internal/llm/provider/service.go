package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

type CredentialStore interface {
	UpsertCredential(ctx context.Context, input CredentialInput) (CredentialRecord, error)
	RevokeCredential(ctx context.Context, owner Scope, label string, actorID string) error
	ListCredentials(ctx context.Context, owner Scope) ([]CredentialRecord, error)
	CredentialForTest(ctx context.Context, owner Scope, label string) (CredentialRecord, error)
	CredentialForResolution(ctx context.Context, owner Scope, credentialID string, providerID ProviderID) (CredentialRecord, error)
	UpdateCredentialTestResult(ctx context.Context, credentialID string, status TestStatus, errorCode TestErrorCode) error
	SelectModelProfile(ctx context.Context, input ModelProfileInput) error
	ActiveModelProfile(ctx context.Context, owner Scope, purpose Purpose) (ModelProfile, error)
}

type Service struct {
	Store    CredentialStore
	Sealer   SecretSealer
	Registry Registry
	Tester   CredentialTester
}

type AddCredentialRequest struct {
	Owner      Scope
	ProviderID ProviderID
	Label      string
	RawSecret  string
	ActorID    string
}

type SelectModelRequest struct {
	Owner        Scope
	Purpose      Purpose
	CredentialID string
	ProviderID   ProviderID
	ModelID      string
	ParamsJSON   string
	ActorID      string
}

type ResolveModelRequest struct {
	Owner   Scope
	Purpose Purpose
	ActorID string
}

type ResolvedModel struct {
	Owner            Scope
	Purpose          Purpose
	ProviderID       ProviderID
	ModelID          string
	ParamsJSON       string
	CredentialID     string
	CredentialLabel  string
	APIKey           string
	BillingOwnerType OwnerType
	BillingOwnerID   string
}

type redactedResolvedModel struct {
	Owner            Scope
	Purpose          Purpose
	ProviderID       ProviderID
	ModelID          string
	ParamsJSON       string
	CredentialID     string
	CredentialLabel  string
	APIKey           SecretValue
	BillingOwnerType OwnerType
	BillingOwnerID   string
}

func (m ResolvedModel) String() string {
	return fmt.Sprintf("%+v", m.redacted())
}

func (m ResolvedModel) GoString() string {
	return fmt.Sprintf("%#v", m.redacted())
}

func (m ResolvedModel) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.redacted())
}

func (m ResolvedModel) LogValue() slog.Value {
	return slog.AnyValue(m.redacted())
}

func (m ResolvedModel) redacted() redactedResolvedModel {
	return redactedResolvedModel{
		Owner:            m.Owner,
		Purpose:          m.Purpose,
		ProviderID:       m.ProviderID,
		ModelID:          m.ModelID,
		ParamsJSON:       m.ParamsJSON,
		CredentialID:     m.CredentialID,
		CredentialLabel:  m.CredentialLabel,
		APIKey:           NewSecretValue(m.APIKey),
		BillingOwnerType: m.BillingOwnerType,
		BillingOwnerID:   m.BillingOwnerID,
	}
}

func NewService(store CredentialStore, sealer SecretSealer, registry Registry) Service {
	return Service{
		Store:    store,
		Sealer:   sealer,
		Registry: registry,
	}
}

func NewServiceWithTester(store CredentialStore, sealer SecretSealer, registry Registry, tester CredentialTester) Service {
	service := NewService(store, sealer, registry)
	service.Tester = tester
	return service
}

func (s Service) AddCredential(ctx context.Context, req AddCredentialRequest) (CredentialRecord, error) {
	if s.Store == nil {
		return CredentialRecord{}, fmt.Errorf("credential store is required")
	}
	if s.Sealer == nil {
		return CredentialRecord{}, fmt.Errorf("secret sealer is required")
	}

	owner, providerID, label, actorID, err := s.normalizeAddCredentialRequest(req)
	if err != nil {
		return CredentialRecord{}, err
	}
	rawSecret := strings.TrimSpace(req.RawSecret)
	fingerprint, err := Fingerprint(rawSecret)
	if err != nil {
		return CredentialRecord{}, err
	}

	sealed, err := s.Sealer.Seal(ctx, []byte(rawSecret), []byte(credentialAAD(owner, providerID, label)))
	if err != nil {
		return CredentialRecord{}, err
	}

	record, err := s.Store.UpsertCredential(ctx, CredentialInput{
		Owner:           owner,
		ProviderID:      providerID,
		Label:           label,
		Ciphertext:      append([]byte(nil), sealed.Ciphertext...),
		Nonce:           append([]byte(nil), sealed.Nonce...),
		KeyID:           strings.TrimSpace(sealed.KeyID),
		Fingerprint:     fingerprint,
		CreatedByUserID: actorID,
		UpdatedByUserID: actorID,
	})
	if err != nil {
		return CredentialRecord{}, err
	}
	return scrubCredentialRecord(record), nil
}

func (s Service) RotateCredential(ctx context.Context, req AddCredentialRequest) (CredentialRecord, error) {
	return s.AddCredential(ctx, req)
}

func (s Service) RevokeCredential(ctx context.Context, owner Scope, label string, actorID string) error {
	if s.Store == nil {
		return fmt.Errorf("credential store is required")
	}

	normalizedOwner, err := normalizeScope(owner)
	if err != nil {
		return err
	}
	label = strings.TrimSpace(label)
	actorID = strings.TrimSpace(actorID)
	if label == "" {
		return fmt.Errorf("credential label is required")
	}
	if actorID == "" {
		return fmt.Errorf("actor user ID is required")
	}
	return s.Store.RevokeCredential(ctx, normalizedOwner, label, actorID)
}

func (s Service) ListCredentials(ctx context.Context, owner Scope) ([]CredentialRecord, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("credential store is required")
	}

	normalizedOwner, err := normalizeScope(owner)
	if err != nil {
		return nil, err
	}
	records, err := s.Store.ListCredentials(ctx, normalizedOwner)
	if err != nil {
		return nil, err
	}
	for i := range records {
		records[i] = scrubCredentialRecord(records[i])
	}
	return records, nil
}

func (s Service) SelectModelProfile(ctx context.Context, req SelectModelRequest) error {
	if s.Store == nil {
		return fmt.Errorf("credential store is required")
	}

	input, err := s.modelProfileInput(req)
	if err != nil {
		return err
	}
	return s.Store.SelectModelProfile(ctx, input)
}

func (s Service) ActiveModelProfile(ctx context.Context, owner Scope, purpose Purpose) (ModelProfile, error) {
	if s.Store == nil {
		return ModelProfile{}, fmt.Errorf("credential store is required")
	}
	return s.Store.ActiveModelProfile(ctx, owner, purpose)
}

func (s Service) TestCredential(ctx context.Context, req TestCredentialRequest) (TestCredentialResult, error) {
	if s.Store == nil {
		return TestCredentialResult{}, fmt.Errorf("credential store is required")
	}
	if s.Sealer == nil {
		return TestCredentialResult{}, fmt.Errorf("secret sealer is required")
	}
	if s.Tester == nil {
		return TestCredentialResult{}, fmt.Errorf("credential tester is required")
	}

	owner, label, err := normalizeTestCredentialRequest(req)
	if err != nil {
		return TestCredentialResult{}, err
	}
	record, err := s.Store.CredentialForTest(ctx, owner, label)
	if err != nil {
		return TestCredentialResult{}, err
	}
	secret, err := s.Sealer.Open(ctx, SealedSecret{
		Ciphertext: append([]byte(nil), record.Ciphertext...),
		Nonce:      append([]byte(nil), record.Nonce...),
		KeyID:      record.KeyID,
	}, []byte(credentialAAD(owner, record.ProviderID, record.Label)))
	if err != nil {
		_ = s.Store.UpdateCredentialTestResult(ctx, record.ID, TestStatusFailed, TestErrorSecretOpenFailed)
		return TestCredentialResult{}, fmt.Errorf("open credential secret: %w", err)
	}

	result, err := s.Tester.TestCredential(ctx, ProviderTestRequest{
		ProviderID: record.ProviderID,
		APIKey:     NewSecretValue(string(secret)),
	})
	if err != nil {
		_ = s.Store.UpdateCredentialTestResult(ctx, record.ID, TestStatusFailed, TestErrorRequestFailed)
		return TestCredentialResult{}, err
	}
	if result.Status == "" {
		result.Status = TestStatusFailed
	}
	result.ProviderID = record.ProviderID
	result.Label = record.Label
	if result.Status == TestStatusSucceeded {
		result.ErrorCode = TestErrorNone
	}
	if err := s.Store.UpdateCredentialTestResult(ctx, record.ID, result.Status, result.ErrorCode); err != nil {
		return TestCredentialResult{}, err
	}
	return result, nil
}

func (s Service) ResolveActiveModel(ctx context.Context, req ResolveModelRequest) (ResolvedModel, error) {
	if s.Store == nil {
		return ResolvedModel{}, fmt.Errorf("credential store is required")
	}
	if s.Sealer == nil {
		return ResolvedModel{}, fmt.Errorf("secret sealer is required")
	}

	owner, purpose, err := normalizeResolveModelRequest(req)
	if err != nil {
		return ResolvedModel{}, err
	}
	profile, err := s.Store.ActiveModelProfile(ctx, owner, purpose)
	if err != nil {
		return ResolvedModel{}, err
	}
	credential, err := s.Store.CredentialForResolution(ctx, owner, profile.CredentialID, profile.ProviderID)
	if err != nil {
		return ResolvedModel{}, err
	}
	secret, err := s.Sealer.Open(ctx, SealedSecret{
		Ciphertext: append([]byte(nil), credential.Ciphertext...),
		Nonce:      append([]byte(nil), credential.Nonce...),
		KeyID:      credential.KeyID,
	}, []byte(credentialAAD(owner, credential.ProviderID, credential.Label)))
	if err != nil {
		return ResolvedModel{}, fmt.Errorf("open credential secret: %w", err)
	}
	return ResolvedModel{
		Owner:            owner,
		Purpose:          purpose,
		ProviderID:       profile.ProviderID,
		ModelID:          profile.ModelID,
		ParamsJSON:       profile.ParamsJSON,
		CredentialID:     credential.ID,
		CredentialLabel:  credential.Label,
		APIKey:           string(secret),
		BillingOwnerType: owner.OwnerType,
		BillingOwnerID:   billingOwnerID(owner),
	}, nil
}

func (s Service) normalizeAddCredentialRequest(req AddCredentialRequest) (Scope, ProviderID, string, string, error) {
	owner, err := normalizeScope(req.Owner)
	if err != nil {
		return Scope{}, "", "", "", err
	}

	providerID := ProviderID(strings.TrimSpace(string(req.ProviderID)))
	if err := s.registry().ValidateProvider(providerID); err != nil {
		return Scope{}, "", "", "", err
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return Scope{}, "", "", "", fmt.Errorf("credential label is required")
	}
	actorID := strings.TrimSpace(req.ActorID)
	if actorID == "" {
		return Scope{}, "", "", "", fmt.Errorf("actor user ID is required")
	}
	return owner, providerID, label, actorID, nil
}

func normalizeTestCredentialRequest(req TestCredentialRequest) (Scope, string, error) {
	owner, err := normalizeScope(req.Owner)
	if err != nil {
		return Scope{}, "", err
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return Scope{}, "", fmt.Errorf("credential label is required")
	}
	actorID := strings.TrimSpace(req.ActorID)
	if actorID == "" {
		return Scope{}, "", fmt.Errorf("actor user ID is required")
	}
	return owner, label, nil
}

func normalizeResolveModelRequest(req ResolveModelRequest) (Scope, Purpose, error) {
	owner, err := normalizeScope(req.Owner)
	if err != nil {
		return Scope{}, "", err
	}
	if err := ValidatePurpose(req.Purpose); err != nil {
		return Scope{}, "", err
	}
	actorID := strings.TrimSpace(req.ActorID)
	if actorID == "" {
		return Scope{}, "", fmt.Errorf("actor user ID is required")
	}
	return owner, req.Purpose, nil
}

func billingOwnerID(owner Scope) string {
	switch owner.OwnerType {
	case OwnerGuild:
		return owner.GuildID
	case OwnerUser:
		return owner.UserID
	default:
		return string(owner.OwnerType)
	}
}

func (s Service) modelProfileInput(req SelectModelRequest) (ModelProfileInput, error) {
	owner, err := normalizeScope(req.Owner)
	if err != nil {
		return ModelProfileInput{}, err
	}
	if err := ValidatePurpose(req.Purpose); err != nil {
		return ModelProfileInput{}, err
	}

	registry := s.registry()
	providerID := ProviderID(strings.TrimSpace(string(req.ProviderID)))
	if err := registry.ValidateProvider(providerID); err != nil {
		return ModelProfileInput{}, err
	}
	modelID, err := ValidateModelID(req.ModelID)
	if err != nil {
		return ModelProfileInput{}, err
	}
	if !registry.SupportsPurpose(providerID, req.Purpose) {
		return ModelProfileInput{}, fmt.Errorf("provider does not support purpose")
	}

	credentialID := strings.TrimSpace(req.CredentialID)
	if credentialID == "" {
		return ModelProfileInput{}, fmt.Errorf("credential ID is required")
	}
	actorID := strings.TrimSpace(req.ActorID)
	if actorID == "" {
		return ModelProfileInput{}, fmt.Errorf("actor user ID is required")
	}
	paramsJSON := strings.TrimSpace(req.ParamsJSON)
	if paramsJSON == "" {
		paramsJSON = "{}"
	}
	if !json.Valid([]byte(paramsJSON)) {
		return ModelProfileInput{}, fmt.Errorf("model profile params must be valid JSON")
	}

	return ModelProfileInput{
		Owner:            owner,
		Purpose:          req.Purpose,
		CredentialID:     credentialID,
		ProviderID:       providerID,
		ModelID:          modelID,
		ParamsJSON:       paramsJSON,
		SelectedByUserID: actorID,
	}, nil
}

func (s Service) registry() Registry {
	if s.Registry.specs == nil {
		return DefaultRegistry()
	}
	return s.Registry
}

func credentialAAD(owner Scope, providerID ProviderID, label string) string {
	return fmt.Sprintf(
		"owner_type=%s;guild_id=%s;user_id=%s;provider_id=%s;label=%s",
		owner.OwnerType,
		owner.GuildID,
		owner.UserID,
		providerID,
		label,
	)
}

func scrubCredentialRecord(record CredentialRecord) CredentialRecord {
	record.Ciphertext = nil
	record.Nonce = nil
	return record
}
