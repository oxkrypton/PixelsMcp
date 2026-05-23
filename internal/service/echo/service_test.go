package echo

import (
	"context"
	"testing"
)

func TestServiceEcho(t *testing.T) {
	svc := NewService()

	got, err := svc.Echo(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Echo returned error: %v", err)
	}

	if got != "hello" {
		t.Fatalf("Echo returned %q, want %q", got, "hello")
	}
}
