package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRuntimeRunsHandlersInOrder(t *testing.T) {
	var calls []string
	runtime := Runtime{Handlers: []Handler{
		HandlerFunc(func(ctx context.Context, request Request) (Response, bool, error) {
			calls = append(calls, "first")
			return Response{}, false, nil
		}),
		HandlerFunc(func(ctx context.Context, request Request) (Response, bool, error) {
			calls = append(calls, "second")
			return Response{Text: "handled"}, true, nil
		}),
		HandlerFunc(func(ctx context.Context, request Request) (Response, bool, error) {
			calls = append(calls, "third")
			return Response{Text: "wrong"}, true, nil
		}),
	}}

	response, err := runtime.Run(context.Background(), Request{Text: " hi "})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if response.Text != "handled" {
		t.Fatalf("response = %+v, want handled", response)
	}
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("calls = %+v, want first then second", calls)
	}
}

func TestRuntimePropagatesHandlerError(t *testing.T) {
	wantErr := errors.New("boom")
	runtime := Runtime{Handlers: []Handler{
		HandlerFunc(func(ctx context.Context, request Request) (Response, bool, error) {
			return Response{}, false, wantErr
		}),
	}}

	_, err := runtime.Run(context.Background(), Request{Text: "hi"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestRuntimeUsesFallback(t *testing.T) {
	runtime := Runtime{
		Handlers: []Handler{HandlerFunc(func(ctx context.Context, request Request) (Response, bool, error) {
			return Response{}, false, nil
		})},
		Fallback: HandlerFunc(func(ctx context.Context, request Request) (Response, bool, error) {
			return Response{Text: "fallback"}, true, nil
		}),
	}

	response, err := runtime.Run(context.Background(), Request{Text: "hi"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if response.Text != "fallback" {
		t.Fatalf("response = %+v, want fallback", response)
	}
}

func TestNormalizeRequestCopiesRoles(t *testing.T) {
	roles := []string{"role-1"}
	request := NormalizeRequest(Request{Surface: " guild_mention ", RoleIDs: roles, Text: " hi "})
	roles[0] = "mutated"

	if request.Surface != SurfaceGuildMention || request.Text != "hi" {
		t.Fatalf("request = %+v, want normalized surface/text", request)
	}
	if got := request.RoleIDs[0]; got != "role-1" {
		t.Fatalf("role = %q, want copy", got)
	}
}
