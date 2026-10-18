package domain

import (
	"fmt"
	"time"
)

type Category string

const (
	CategoryFood          Category = "еда"
	CategoryTransport     Category = "транспорт"
	CategoryHousing       Category = "жильё"
	CategoryEntertainment Category = "развлечения"
	CategoryHealth        Category = "здоровье"
	CategoryShopping      Category = "покупки"
	CategoryAlcohol       Category = "алкоголь"
	CategoryOther         Category = "другое"
)

var ValidCategories = []Category{
	CategoryFood,
	CategoryTransport,
	CategoryHousing,
	CategoryEntertainment,
	CategoryHealth,
	CategoryShopping,
	CategoryAlcohol,
	CategoryOther,
}

func (c Category) Valid() bool {
	for _, valid := range ValidCategories {
		if c == valid {
			return true
		}
	}
	return false
}

type Expense struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Amount    float64   `json:"amount"`
	Category  Category  `json:"category"`
	SpentAt   time.Time `json:"spent_at"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExpenseFilter — параметры фильтрации списка трат.
type ExpenseFilter struct {
	UserID   *int64
	Category *Category
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
}

const (
	DefaultLimit = 50
	MaxLimit     = 500
)

// Normalize применяет значения по умолчанию и ограничения к фильтру.
func (f *ExpenseFilter) Normalize() {
	if f.Limit <= 0 || f.Limit > MaxLimit {
		f.Limit = DefaultLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// ExpenseInput — данные для создания/обновления траты.
type ExpenseInput struct {
	Amount   float64   `json:"amount"`
	Category Category  `json:"category"`
	SpentAt  time.Time `json:"spent_at"`
	Comment  string    `json:"comment,omitempty"`
}

// Validate проверяет корректность входных данных.
func (i ExpenseInput) Validate() error {
	if i.Amount <= 0 {
		return fmt.Errorf("%w: amount=%.2f", ErrInvalidAmount, i.Amount)
	}
	if !i.Category.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidCategory, i.Category)
	}
	return nil
}
