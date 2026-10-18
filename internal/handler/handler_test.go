package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dhiliphari/shortner/internal/handler"
	"github.com/dhiliphari/shortner/internal/model"
	"github.com/dhiliphari/shortner/internal/repository"
	"github.com/dhiliphari/shortner/internal/service"
)

const testBaseURL = "http://short.test"

func newTestHandler(t *testing.T) (http.Handler, *service.LinkService) {
	t.Helper()
	svc := service.New(repository.NewMemoryRepository())
	return handler.New(svc, testBaseURL).Routes(), svc
}

func TestShorten(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"valid url", `{"url":"https://go.dev"}`, http.StatusCreated},
		{"invalid json", `{"url":`, http.StatusBadRequest},
		{"empty body", ``, http.StatusBadRequest},
		{"missing url field", `{}`, http.StatusBadRequest},
		{"not a url", `{"url":"not-a-url"}`, http.StatusBadRequest},
		{"unsupported schema", `{"url":"fpt://example.com"}`, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler(t)

			req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			if rec.Code != http.StatusCreated {
				return
			}

			var resp struct {
				Code     string `json:"code"`
				ShortURL string `json:"short_url"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if codeLength := len(resp.Code); codeLength != 7 {
				t.Errorf("code length = %d, want 7", codeLength)
			}
			if want := testBaseURL + "/" + resp.Code; resp.ShortURL != want {
				t.Errorf("short_url = %q, want %q", resp.ShortURL, want)
			}
		})
	}
}

func TestShorten_Alias(t *testing.T) {
	h, _ := newTestHandler(t)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := post(`{"url":"https://go.dev", "alias":"golang"}`); rec.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d, want 201", rec.Code)
	}
	if rec := post(`{"url":"https://example.com", "alias":"golang"}`); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate alias: status = %d, want 409", rec.Code)
	}
	if rec := post(`{"url":"https://example.com", "alias":"a b!"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad alias: status = %d, want 400", rec.Code)
	}
}

func TestRedirect(t *testing.T) {
	h, svc := newTestHandler(t)

	link, err := svc.Shorten(context.Background(), "https://go.dev", "")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/"+link.Code, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != "https://go.dev" {
		t.Errorf("location = %q, want %q", loc, "https://go.dev")
	}
}

func TestRedirect_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/doesnotexist", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestStats_CountsRedirects(t *testing.T) {
	h, svc := newTestHandler(t)

	link, err := svc.Shorten(context.Background(), "https://go.dev", "")
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		svc.Resolve(context.Background(), link.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/stats/"+link.Code, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got model.Link
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Clicks != 3 {
		t.Errorf("clicks = %d, want 3", got.Clicks)
	}
	if got.URL != "https://go.dev" {
		t.Errorf("url = %q, want https://go.dev", got.URL)
	}
}

func TestEndToEnd(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h)
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 1. Create a short link
	resp, err := client.Post(srv.URL+"/shorten", "application/json", strings.NewReader(`{"url":"https://go.dev"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var created struct{ Code string }
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	// 2. Follow it
	resp2, err := client.Get(srv.URL + "/" + created.Code)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp2.StatusCode)
	}
	if loc := resp2.Header.Get("Location"); loc != "https://go.dev" {
		t.Errorf("Location = %q, want https://go.dev", loc)
	}
}
