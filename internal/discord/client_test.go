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
	opens      int
	closes     int
	openErr    error
	closeErr   error
	closeBlock chan struct{}
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
