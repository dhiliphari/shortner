package repository

import (
	"context"
	"sync"

	"github.com/dhiliphari/shortner/internal/model"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	links map[string]model.Link
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		links: make(map[string]model.Link),
	}
}

func (r *MemoryRepository) Create(ctx context.Context, link model.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.links[link.Code]; exists {
		return ErrCodeExists
	}

	r.links[link.Code] = link
	return nil
}

func (r *MemoryRepository) Get(ctx context.Context, code string) (model.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	link, ok := r.links[code]
	if !ok {
		return model.Link{}, ErrNotFound
	}

	return link, nil
}

func (r *MemoryRepository) IncrementClicks(ctx context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	link, ok := r.links[code]
	if !ok {
		return ErrNotFound
	}
	link.Clicks++
	r.links[code] = link
	return nil
}
