package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/dhiliphari/shortner/internal/repository"
	"github.com/dhiliphari/shortner/internal/service"
)

func TestShorten_Validation(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr error
		alias   string
	}{
		{"valid https", "https://example.com/path?q=1", nil, ""},
		{"valid http", "http://example.com", nil, ""},
		{"missing scheme", "example.com", service.ErrInvalidURL, ""},
		{"unsupported scheme", "ftp://example.com", service.ErrInvalidURL, ""},
		{"no host", "https://", service.ErrInvalidURL, ""},
		{"empty", "", service.ErrInvalidURL, ""},

		{"valid alias", "https://example.com", nil, "example"},
		{"short alias", "https://example.com", service.ErrInvalidAlias, "ex"},
		{"long alias", "https://example.com", service.ErrInvalidAlias, strings.Repeat("a", 40)},
		{"invalid alias", "https://example.com", service.ErrInvalidAlias, "abc@123#"},
	}

	svc := service.New(repository.NewMemoryRepository())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Shorten(context.Background(), tc.url, tc.alias)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestResolve_CountsClick(t *testing.T) {
	ctx := context.Background()
	svc := service.New(repository.NewMemoryRepository())

	link, err := svc.Shorten(ctx, "https://test.com", "")
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if _, err := svc.Resolve(ctx, link.Code); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := svc.Stats(ctx, link.Code)
	if err != nil {
		t.Fatal(err)
	}

	if stats.Clicks != 3 {
		t.Errorf("clicks = %d, want 3", stats.Clicks)
	}
}

func TestResolve_NotFound(t *testing.T) {
	svc := service.New(repository.NewMemoryRepository())
	_, err := svc.Resolve(context.Background(), "nope")
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestConcurrentResolve_Stats(t *testing.T) {
	svc := service.New(repository.NewMemoryRepository())

	link, err := svc.Shorten(context.Background(), "https://test.com", "")
	if err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	for range 100 {
		wg.Go(func() {
			if _, err := svc.Resolve(context.Background(), link.Code); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	link, err = svc.Stats(context.Background(), link.Code)
	if err != nil {
		t.Fatal(err)
	}

	if link.Clicks != 100 {
		t.Errorf("clicks = %d, expected 100", link.Clicks)
	}
}
