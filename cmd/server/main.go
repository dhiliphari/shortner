package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dhiliphari/shortner/internal/handler"
	"github.com/dhiliphari/shortner/internal/repository"
	"github.com/dhiliphari/shortner/internal/service"
)

func main() {
	addr := ":5000"
	baseURL := "http://localhost:5000"

	repo := repository.NewMemoryRepository()
	svc := service.New(repo)
	h := handler.New(svc, baseURL)

	srv := &http.Server{
		Addr:              addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
