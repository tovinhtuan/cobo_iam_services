package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeListedRepo struct {
	listFn func(context.Context, ListParams) (ListResult, error)
	getFn  func(context.Context, string) (ListedCompanyDetail, error)
}

func (f *fakeListedRepo) List(ctx context.Context, p ListParams) (ListResult, error) {
	if f.listFn != nil {
		return f.listFn(ctx, p)
	}
	return ListResult{}, nil
}

func (f *fakeListedRepo) GetBySymbol(ctx context.Context, symbol string) (ListedCompanyDetail, error) {
	if f.getFn != nil {
		return f.getFn(ctx, symbol)
	}
	return ListedCompanyDetail{}, nil
}

func TestService_DisabledUnavailable(t *testing.T) {
	svc := NewDisabledService()
	_, err := svc.List(context.Background(), ListParams{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("List err = %v, want ErrUnavailable", err)
	}
	_, err = svc.GetDetail(context.Background(), "VIC")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("GetDetail err = %v, want ErrUnavailable", err)
	}
}

func TestService_PingFailureUnavailable(t *testing.T) {
	svc := NewService(&fakeListedRepo{}, func(context.Context) error {
		return errors.New("db down")
	})
	_, err := svc.List(context.Background(), ListParams{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("List err = %v", err)
	}
}

func TestService_GetDetailNotFound(t *testing.T) {
	svc := NewService(&fakeListedRepo{
		getFn: func(context.Context, string) (ListedCompanyDetail, error) {
			return ListedCompanyDetail{}, ErrNotFound
		},
	}, nil)
	_, err := svc.GetDetail(context.Background(), "ZZZ")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestService_ListSuccess(t *testing.T) {
	now := time.Now().UTC()
	svc := NewService(&fakeListedRepo{
		listFn: func(_ context.Context, p ListParams) (ListResult, error) {
			if p.Limit != 20 {
				t.Fatalf("limit = %d", p.Limit)
			}
			return ListResult{
				Items: []ListedCompanySummary{{Symbol: "VIC", CompanyName: "Vingroup", Source: ProfileSourceKBS, HasProfile: true, ProfileUpdatedAt: &now}},
				Total: 1,
			}, nil
		},
	}, nil)
	out, err := svc.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Items[0].Symbol != "VIC" {
		t.Fatalf("got %+v", out)
	}
}
