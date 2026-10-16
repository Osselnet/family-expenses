package service

import (
	"context"
	"fmt"
	"time"

	"family-expenses/internal/domain"
	"family-expenses/internal/repository"
)

type ExpenseService struct {
	repo repository.ExpenseRepository
}

func NewExpenseService(repo repository.ExpenseRepository) *ExpenseService {
	return &ExpenseService{repo: repo}
}

func (s *ExpenseService) Add(ctx context.Context, userID int64, input domain.ExpenseInput) (*domain.Expense, error) {
	if input.SpentAt.IsZero() {
		input.SpentAt = time.Now()
	}
	expense, err := s.repo.Create(ctx, userID, input)
	if err != nil {
		return nil, fmt.Errorf("add expense: %w", err)
	}
	return expense, nil
}

func (s *ExpenseService) Get(ctx context.Context, userID int64, id string) (*domain.Expense, error) {
	return s.repo.GetByID(ctx, userID, id)
}

func (s *ExpenseService) Update(ctx context.Context, userID int64, id string, input domain.ExpenseInput) (*domain.Expense, error) {
	return s.repo.Update(ctx, userID, id, input)
}

func (s *ExpenseService) Delete(ctx context.Context, userID int64, id string) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *ExpenseService) List(ctx context.Context, filter domain.ExpenseFilter) ([]*domain.Expense, error) {
	return s.repo.List(ctx, filter)
}

func (s *ExpenseService) GetLast(ctx context.Context, userID int64) (*domain.Expense, error) {
	return s.repo.GetLast(ctx, userID)
}

// ListAll — для веб-панели главы семьи (все траты без фильтра по пользователю).
func (s *ExpenseService) ListAll(ctx context.Context, filter domain.ExpenseFilter) ([]*domain.Expense, error) {
	filter.UserID = nil // сброс фильтра по пользователю
	return s.repo.List(ctx, filter)
}

// GetByIDAll — получение траты без фильтрации по user_id (веб-панель).
func (s *ExpenseService) GetByIDAll(ctx context.Context, id string) (*domain.Expense, error) {
	return s.repo.GetByIDAll(ctx, id)
}

// UpdateAll — обновление траты без фильтрации по user_id (веб-панель).
func (s *ExpenseService) UpdateAll(ctx context.Context, id string, input domain.ExpenseInput) (*domain.Expense, error) {
	return s.repo.UpdateAll(ctx, id, input)
}

// DeleteAll — удаление траты без фильтрации по user_id (веб-панель).
func (s *ExpenseService) DeleteAll(ctx context.Context, id string) error {
	return s.repo.DeleteAll(ctx, id)
}

// FormatExpense — форматирование траты для ответа пользователю.
func FormatExpense(e *domain.Expense) string {
	date := e.SpentAt.Format("02.01.2006")
	if e.Comment != "" {
		return fmt.Sprintf("💰 %s: %.0f ₽ (%s), %s", e.Comment, e.Amount, e.Category, date)
	}
	return fmt.Sprintf("💰 %.0f ₽ (%s), %s", e.Amount, e.Category, date)
}

// FormatExpenseList — форматирование списка трат.
func FormatExpenseList(expenses []*domain.Expense) string {
	if len(expenses) == 0 {
		return "Траты не найдены."
	}

	total := 0.0
	result := ""
	for _, e := range expenses {
		result += FormatExpense(e) + "\n"
		total += e.Amount
	}
	result += fmt.Sprintf("\n📊 Итого: %.0f ₽ за %d записей", total, len(expenses))
	return result
}