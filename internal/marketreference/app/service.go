package app

import (
	"context"
	"errors"
)

// Service exposes read-only listed company reference operations for CMS.
type Service struct {
	repo ListedCompanyReader
	ping func(context.Context) error
}

// NewService wires a vnstock reader with optional DB ping (503 when ping fails).
func NewService(repo ListedCompanyReader, ping func(context.Context) error) *Service {
	return &Service{repo: repo, ping: ping}
}

// NewDisabledService returns a service that always responds with ErrUnavailable.
func NewDisabledService() *Service {
	return &Service{}
}

// List returns a filtered page of listed companies.
func (s *Service) List(ctx context.Context, p ListParams) (ListResult, error) {
	if err := s.checkAvailable(ctx); err != nil {
		return ListResult{}, err
	}
	p, err := NormalizeListParams(p)
	if err != nil {
		return ListResult{}, err
	}
	out, err := s.repo.List(ctx, p)
	if err != nil {
		return ListResult{}, mapRepoError(err)
	}
	return out, nil
}

// GetDetail returns company detail; partial when equity exists without profile.
func (s *Service) GetDetail(ctx context.Context, symbol string) (ListedCompanyDetail, error) {
	if err := s.checkAvailable(ctx); err != nil {
		return ListedCompanyDetail{}, err
	}
	symbol = NormalizeSymbol(symbol)
	if symbol == "" {
		return ListedCompanyDetail{}, ErrInvalidRequest
	}
	detail, err := s.repo.GetBySymbol(ctx, symbol)
	if err != nil {
		return ListedCompanyDetail{}, mapRepoError(err)
	}
	return detail, nil
}

func (s *Service) checkAvailable(ctx context.Context) error {
	if s.repo == nil {
		return ErrUnavailable
	}
	if s.ping != nil {
		if err := s.ping(ctx); err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

func mapRepoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrNotFound) {
		return err
	}
	return ErrUnavailable
}
