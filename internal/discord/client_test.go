package discord

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestNewGatewayNormalizesBotTokenAndIntents(t *testing.T) {
	var gotToken string
	var gotIntents discordgo.Intent

	gateway, err := newGatewayWithFactory(Options{Token: " raw-token "}, func(token string, intents discordgo.Intent) (gatewaySession, error) {
		gotToken = token
		gotIntents = intents
		return &fakeSession{}, nil
	})
	if err != nil {
		t.Fatalf("newGatewayWithFactory returned error: %v", err)
	}
	if gateway == nil {
		t.Fatal("gateway is nil")
	}
	if gotToken != "Bot raw-token" {
		t.Fatalf("token = %q, want Bot raw-token", gotToken)
	}
	if gotIntents != DefaultIntents() {
		t.Fatalf("intents = %v, want %v", gotIntents, DefaultIntents())
	}
}

func TestNewGatewayKeepsExplicitBotPrefix(t *testing.T) {
	var gotToken string

	_, err := newGatewayWithFactory(Options{Token: "Bot raw-token"}, func(token string, intents discordgo.Intent) (gatewaySession, error) {
		gotToken = token
		return &fakeSession{}, nil
	})
	if err != nil {
		t.Fatalf("newGatewayWithFactory returned error: %v", err)
	}
	if gotToken != "Bot raw-token" {
		t.Fatalf("token = %q, want Bot raw-token", gotToken)
	}
}

func TestNewGatewayRequiresToken(t *testing.T) {
	_, err := newGatewayWithFactory(Options{Token: " "}, func(token string, intents discordgo.Intent) (gatewaySession, error) {
		return &fakeSession{}, nil
	})
	if err == nil {
		t.Fatal("expected token error")
	}
}

func TestNewGatewayRequiresClientIDWhenCommandSyncEnabled(t *testing.T) {
	_, err := newGatewayWithFactory(Options{Token: "token", SyncCommands: true}, func(token string, intents discordgo.Intent) (gatewaySession, error) {
		return &fakeSession{}, nil
	})
	if err == nil {
		t.Fatal("expected client ID error")
	}
}

func TestGatewayStartAndClose(t *testing.T) {
	session := &fakeSession{}
	gateway := &Gateway{session: session}

	if err := gateway.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := gateway.Start(context.Background()); err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if session.opens != 1 {
		t.Fatalf("opens = %d, want 1", session.opens)
	}

	if err := gateway.Close(context.Background()); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := gateway.Close(context.Background()); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
	if session.closes != 1 {
		t.Fatalf("closes = %d, want 1", session.closes)
	}
}

func TestGatewaySyncsApplicationCommands(t *testing.T) {
	router, err := NewCommandRouter(CoreCommands()...)
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	session := &fakeSession{}
	gateway := &Gateway{
		session:       session,
		clientID:      "client-id",
		guildID:       "guild-id",
		syncCommands:  true,
		commandRouter: router,
	}

	if err := gateway.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if session.bulkAppID != "client-id" {
		t.Fatalf("bulk app ID = %q, want client-id", session.bulkAppID)
	}
	if session.bulkGuildID != "guild-id" {
		t.Fatalf("bulk guild ID = %q, want guild-id", session.bulkGuildID)
	}
	if len(session.bulkCommands) != 1 || session.bulkCommands[0].Name != "ping" {
		t.Fatalf("bulk commands = %+v", session.bulkCommands)
	}
}

func TestGatewayClosesWhenCommandSyncFails(t *testing.T) {
	router, err := NewCommandRouter(CoreCommands()...)
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	session := &fakeSession{bulkErr: errors.New("boom")}
	gateway := &Gateway{
		session:       session,
		clientID:      "client-id",
		syncCommands:  true,
		commandRouter: router,
	}

	err = gateway.Start(context.Background())
	if err == nil {
		t.Fatal("expected command sync error")
	}
	if session.opens != 1 {
		t.Fatalf("opens = %d, want 1", session.opens)
	}
	if session.closes != 1 {
		t.Fatalf("closes = %d, want 1", session.closes)
	}
}

func TestNewGatewayRegistersCommandRouter(t *testing.T) {
	router, err := NewCommandRouter(CoreCommands()...)
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	session := &fakeSession{}

	_, err = newGatewayWithFactory(Options{Token: "token", CommandRouter: router}, func(token string, intents discordgo.Intent) (gatewaySession, error) {
		return session, nil
	})
	if err != nil {
		t.Fatalf("newGatewayWithFactory returned error: %v", err)
	}
	if len(session.handlers) != 1 {
		t.Fatalf("handlers = %d, want 1", len(session.handlers))
	}
}

func TestNewGatewayRegistersMessageRouter(t *testing.T) {
	router, err := NewMessageRouter("bot-id", CoreMessageHandler(), nil)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}
	session := &fakeSession{}

	_, err = newGatewayWithFactory(Options{Token: "token", MessageRouter: router}, func(token string, intents discordgo.Intent) (gatewaySession, error) {
		return session, nil
	})
	if err != nil {
		t.Fatalf("newGatewayWithFactory returned error: %v", err)
	}
	if len(session.handlers) != 1 {
		t.Fatalf("handlers = %d, want 1", len(session.handlers))
	}
}

func TestGatewayStartReturnsOpenError(t *testing.T) {
	gateway := &Gateway{session: &fakeSession{openErr: errors.New("boom")}}

	err := gateway.Start(context.Background())
	if err == nil {
		t.Fatal("expected open error")
	}
}

func TestGatewayCloseHonorsContext(t *testing.T) {
	session := &fakeSession{closeBlock: make(chan struct{})}
	gateway := &Gateway{session: session, started: true}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err := gateway.Close(ctx)
	if err == nil {
		t.Fatal("expected context error")
	}
	close(session.closeBlock)
}

type fakeSession struct {
	opens        int
	closes       int
	openErr      error
	closeErr     error
	closeBlock   chan struct{}
	handlers     []interface{}
	bulkAppID    string
	bulkGuildID  string
	bulkCommands []*discordgo.ApplicationCommand
	bulkErr      error
}

func (s *fakeSession) AddHandler(handler interface{}) func() {
	s.handlers = append(s.handlers, handler)
	return func() {}
}

func (s *fakeSession) ApplicationCommandBulkOverwrite(appID string, guildID string, commands []*discordgo.ApplicationCommand, _ ...discordgo.RequestOption) ([]*discordgo.ApplicationCommand, error) {
	s.bulkAppID = appID
	s.bulkGuildID = guildID
	s.bulkCommands = commands
	return commands, s.bulkErr
}

func (s *fakeSession) Open() error {
	s.opens++
	return s.openErr
}

func (s *fakeSession) Close() error {
	s.closes++
	if s.closeBlock != nil {
		<-s.closeBlock
	}
	return s.closeErr
}
