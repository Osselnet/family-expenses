package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"family-expenses/internal/domain"
)

type ExpenseRepository struct {
	db *pgxpool.Pool
}

func NewExpenseRepository(db *pgxpool.Pool) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

func (r *ExpenseRepository) Create(ctx context.Context, userID int64, input domain.ExpenseInput) (*domain.Expense, error) {
	if input.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if !input.Category.Valid() {
		return nil, domain.ErrInvalidCategory
	}

	var e domain.Expense
	err := r.db.QueryRow(ctx, `
		INSERT INTO expenses (user_id, amount, category, spent_at, comment)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, amount, category, spent_at, comment, created_at, updated_at
	`, userID, input.Amount, input.Category, input.SpentAt, input.Comment).
		Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert expense: %w", err)
	}
	return &e, nil
}

func (r *ExpenseRepository) GetByID(ctx context.Context, userID int64, id string) (*domain.Expense, error) {
	var e domain.Expense
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, amount, category, spent_at, comment, created_at, updated_at
		FROM expenses WHERE id = $1 AND user_id = $2
	`, id, userID).
		Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get expense: %w", err)
	}
	return &e, nil
}

func (r *ExpenseRepository) Update(ctx context.Context, userID int64, id string, input domain.ExpenseInput) (*domain.Expense, error) {
	if input.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if !input.Category.Valid() {
		return nil, domain.ErrInvalidCategory
	}

	var e domain.Expense
	err := r.db.QueryRow(ctx, `
		UPDATE expenses
		SET amount = $3, category = $4, spent_at = $5, comment = $6, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, amount, category, spent_at, comment, created_at, updated_at
	`, id, userID, input.Amount, input.Category, input.SpentAt, input.Comment).
		Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update expense: %w", err)
	}
	return &e, nil
}

func (r *ExpenseRepository) Delete(ctx context.Context, userID int64, id string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM expenses WHERE id = $1 AND user_id = $2
	`, id, userID)

	if err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ExpenseRepository) List(ctx context.Context, filter domain.ExpenseFilter) ([]*domain.Expense, error) {
	query := `
		SELECT id, user_id, amount, category, spent_at, comment, created_at, updated_at
		FROM expenses WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.Category != nil {
		query += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, *filter.Category)
		argIdx++
	}
	if filter.DateFrom != nil {
		query += fmt.Sprintf(" AND spent_at >= $%d", argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		query += fmt.Sprintf(" AND spent_at <= $%d", argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}

	query += " ORDER BY spent_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	var expenses []*domain.Expense
	for rows.Next() {
		var e domain.Expense
		if err := rows.Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan expense: %w", err)
		}
		expenses = append(expenses, &e)
	}
	return expenses, nil
}

func (r *ExpenseRepository) GetLast(ctx context.Context, userID int64) (*domain.Expense, error) {
	var e domain.Expense
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, amount, category, spent_at, comment, created_at, updated_at
		FROM expenses WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, userID).
		Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get last expense: %w", err)
	}
	return &e, nil
}

// GetByIDAll — получение траты без фильтрации по user_id (для веб-панели).
func (r *ExpenseRepository) GetByIDAll(ctx context.Context, id string) (*domain.Expense, error) {
	var e domain.Expense
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, amount, category, spent_at, comment, created_at, updated_at
		FROM expenses WHERE id = $1
	`, id).
		Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get expense all: %w", err)
	}
	return &e, nil
}

// UpdateAll — обновление траты без фильтрации по user_id (для веб-панели).
func (r *ExpenseRepository) UpdateAll(ctx context.Context, id string, input domain.ExpenseInput) (*domain.Expense, error) {
	if input.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if !input.Category.Valid() {
		return nil, domain.ErrInvalidCategory
	}

	var e domain.Expense
	err := r.db.QueryRow(ctx, `
		UPDATE expenses
		SET amount = $2, category = $3, spent_at = $4, comment = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, amount, category, spent_at, comment, created_at, updated_at
	`, id, input.Amount, input.Category, input.SpentAt, input.Comment).
		Scan(&e.ID, &e.UserID, &e.Amount, &e.Category, &e.SpentAt, &e.Comment, &e.CreatedAt, &e.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update expense all: %w", err)
	}
	return &e, nil
}

// DeleteAll — удаление траты без фильтрации по user_id (для веб-панели).
func (r *ExpenseRepository) DeleteAll(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM expenses WHERE id = $1
	`, id)

	if err != nil {
		return fmt.Errorf("delete expense all: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Migrate — применяет миграции при старте.
func (r *ExpenseRepository) Migrate(ctx context.Context) error {
	schema := `
		CREATE TABLE IF NOT EXISTS expenses (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id BIGINT NOT NULL,
			amount DECIMAL(10,2) NOT NULL CHECK (amount > 0),
			category VARCHAR(32) NOT NULL CHECK (category IN (
				'еда', 'транспорт', 'жильё', 'развлечения',
				'здоровье', 'покупки', 'алкоголь', 'другое'
			)),
			spent_at TIMESTAMP NOT NULL,
			comment TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_expenses_user_spent ON expenses(user_id, spent_at DESC);
		CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category);
	`
	_, err := r.db.Exec(ctx, schema)
	return err
}

// Утилита для сборки WHERE
func buildWhere(filters map[string]interface{}) (string, []interface{}) {
	conditions := []string{}
	args := []interface{}{}
	idx := 1
	for field, value := range filters {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", field, idx))
		args = append(args, value)
		idx++
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	return where, args
}