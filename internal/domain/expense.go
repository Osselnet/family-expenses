package domain

import (
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
	CategoryOther         Category = "другое"
)

var ValidCategories = []Category{
	CategoryFood,
	CategoryTransport,
	CategoryHousing,
	CategoryEntertainment,
	CategoryHealth,
	CategoryShopping,
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

// ExpenseInput — данные для создания/обновления траты.
type ExpenseInput struct {
	Amount   float64  `json:"amount"`
	Category Category `json:"category"`
	SpentAt  time.Time `json:"spent_at"`
	Comment  string   `json:"comment,omitempty"`
}