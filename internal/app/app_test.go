package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/gOps132/GigiDC/internal/agent"
	"github.com/gOps132/GigiDC/internal/config"
	"github.com/gOps132/GigiDC/internal/llm"
)

func TestRunStartsAndShutdownClosesDiscordClient(t *testing.T) {
	discordClient := newFakeDiscordClient()
	application, err := New(
		config.Config{
			Env:             "test",
			HTTPAddr:        "127.0.0.1:0",
			DatabaseURL:     "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable",
			DiscordEnabled:  true,
			DiscordToken:    "token",
			DiscordClientID: "client-id",
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithReadyCheck(func(context.Context) error { return nil }),
		WithDiscordClient(discordClient),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	select {
	case <-discordClient.started:
	case <-time.After(time.Second):
		t.Fatal("discord client did not start")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancel")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	select {
	case <-discordClient.closed:
	case <-time.After(time.Second):
		t.Fatal("discord client did not close")
	}
}

func TestRunClosesDiscordClientWhenHTTPFails(t *testing.T) {
	discordClient := newFakeDiscordClient()
	application, err := New(
		config.Config{
			Env:             "test",
			HTTPAddr:        "bad address",
			DatabaseURL:     "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable",
			DiscordEnabled:  true,
			DiscordToken:    "token",
			DiscordClientID: "client-id",
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithReadyCheck(func(context.Context) error { return nil }),
		WithDiscordClient(discordClient),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	err = application.Run(context.Background())
	if err == nil {
		t.Fatal("expected HTTP bind error")
	}

	select {
	case <-discordClient.started:
	default:
		t.Fatal("discord client did not start")
	}

	select {
	case <-discordClient.closed:
	case <-time.After(time.Second):
		t.Fatal("discord client did not close after HTTP failure")
	}
}

func TestGuildMentionPlannerStartsWithWebIntentPlanner(t *testing.T) {
	planner, ok := guildMentionPlanner(llm.Runtime{}).(agent.MultiPlanner)
	if !ok {
		t.Fatalf("planner type = %T, want agent.MultiPlanner", guildMentionPlanner(llm.Runtime{}))
	}
	if len(planner) != 3 {
		t.Fatalf("planner length = %d, want web intent, LLM, and semantic memory planners", len(planner))
	}
	if _, ok := planner[0].(agent.WebIntentPlanner); !ok {
		t.Fatalf("first planner = %T, want agent.WebIntentPlanner", planner[0])
	}
	if _, ok := planner[1].(agent.LLMPlanner); !ok {
		t.Fatalf("second planner = %T, want agent.LLMPlanner", planner[1])
	}
	if _, ok := planner[2].(agent.SemanticMemoryPlannerAdapter); !ok {
		t.Fatalf("third planner = %T, want agent.SemanticMemoryPlannerAdapter", planner[2])
	}
}

type fakeDiscordClient struct {
	started chan struct{}
	closed  chan struct{}
}

func newFakeDiscordClient() *fakeDiscordClient {
	return &fakeDiscordClient{
		started: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (c *fakeDiscordClient) Start(context.Context) error {
	close(c.started)
	return nil
}

func (c *fakeDiscordClient) Close(context.Context) error {
	close(c.closed)
	return nil
}
