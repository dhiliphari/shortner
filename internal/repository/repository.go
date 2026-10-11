package repository

import (
	"context"
	"errors"

	"github.com/dhiliphari/shortner/internal/model"
)

var (
	ErrNotFound   = errors.New("link not found")
	ErrCodeExists = errors.New("code already exists")
)

type LinkRepository interface {
	Create(ctx context.Context, link model.Link) error
	Get(ctx context.Context, code string) (model.Link, error)
	IncrementClicks(ctx context.Context, code string) error
}
