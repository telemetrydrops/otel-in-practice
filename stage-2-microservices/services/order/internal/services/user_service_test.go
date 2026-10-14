package services

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"gorm.io/gorm"
)

type mockUserRepo struct {
	createFn  func(ctx context.Context, u *models.User) error
	getByIDFn func(ctx context.Context, id string) (*models.User, error)
}

func (m *mockUserRepo) Create(ctx context.Context, u *models.User) error {
	return m.createFn(ctx, u)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	return m.getByIDFn(ctx, id)
}

func newTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(os.Stderr, nil)) }

func TestUserService_Register_AssignsIDAndPersists(t *testing.T) {
	var captured *models.User
	repo := &mockUserRepo{createFn: func(_ context.Context, u *models.User) error {
		captured = u
		return nil
	}}
	svc, err := NewUserService(repo, newTestLogger())
	if err != nil {
		t.Fatalf("NewUserService: %v", err)
	}
	got, err := svc.Register(context.Background(), "alice@example.com", "premium")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected ID to be assigned")
	}
	if captured == nil || captured.Email != "alice@example.com" || captured.Tier != "premium" {
		t.Fatalf("unexpected captured user: %+v", captured)
	}
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	repo := &mockUserRepo{getByIDFn: func(_ context.Context, _ string) (*models.User, error) {
		return nil, gorm.ErrRecordNotFound
	}}
	svc, err := NewUserService(repo, newTestLogger())
	if err != nil {
		t.Fatalf("NewUserService: %v", err)
	}
	_, err = svc.GetUser(context.Background(), "missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("got err=%v, want ErrUserNotFound", err)
	}
}
