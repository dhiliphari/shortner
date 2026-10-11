package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"time"

	"github.com/dhiliphari/shortner/internal/model"
	"github.com/dhiliphari/shortner/internal/repository"
)

const (
	alphabet   = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	codeLength = 7
	maxRetries = 5
)

var (
	ErrInvalidURL = errors.New("invalid url")
	ErrNotFound   = repository.ErrNotFound
)

type LinkService struct {
	repo repository.LinkRepository
}

func New(repo repository.LinkRepository) *LinkService {
	return &LinkService{repo: repo}
}

func (s *LinkService) Shorten(ctx context.Context, rawURL string) (model.Link, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return model.Link{}, ErrInvalidURL
	}

	for range maxRetries {
		code, err := generateCode()
		if err != nil {
			return model.Link{}, fmt.Errorf("generate code: %w", err)
		}

		link := model.Link{Code: code, URL: rawURL, CreatedAt: time.Now().UTC()}
		err = s.repo.Create(ctx, link)
		if errors.Is(err, repository.ErrCodeExists) {
			continue // collision, try another code
		}
		if err != nil {
			return model.Link{}, fmt.Errorf("create link: %w", err)
		}
		return link, nil
	}
	return model.Link{}, errors.New("could not generate a unique code")
}

// Resolve returns the original URL and records the click.
func (s *LinkService) Resolve(ctx context.Context, code string) (string, error) {
	link, err := s.repo.Get(ctx, code)
	if err != nil {
		return "", err
	}
	if err := s.repo.IncrementClicks(ctx, code); err != nil {
		return "", err
	}
	return link.URL, nil
}

func (s *LinkService) Stats(ctx context.Context, code string) (model.Link, error) {
	return s.repo.Get(ctx, code)
}

func generateCode() (string, error) {
	b := make([]byte, codeLength)
	max := big.NewInt(int64(len(alphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}
