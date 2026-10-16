package repository

import (
	"context"
	"family-expenses/internal/domain"
)

// ExpenseRepository — интерфейс для работы с тратами.
type ExpenseRepository interface {
	Create(ctx context.Context, userID int64, input domain.ExpenseInput) (*domain.Expense, error)
	GetByID(ctx context.Context, userID int64, id string) (*domain.Expense, error)
	Update(ctx context.Context, userID int64, id string, input domain.ExpenseInput) (*domain.Expense, error)
	Delete(ctx context.Context, userID int64, id string) error
	List(ctx context.Context, filter domain.ExpenseFilter) ([]*domain.Expense, error)
	GetLast(ctx context.Context, userID int64) (*domain.Expense, error)
	// Методы без фильтрации по user_id — для веб-панели главы семьи.
	GetByIDAll(ctx context.Context, id string) (*domain.Expense, error)
	UpdateAll(ctx context.Context, id string, input domain.ExpenseInput) (*domain.Expense, error)
	DeleteAll(ctx context.Context, id string) error
}