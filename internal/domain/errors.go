package domain

import "errors"

var (
	ErrNotFound      = errors.New("запись не найдена")
	ErrInvalidAmount = errors.New("сумма должна быть положительной")
	ErrInvalidCategory = errors.New("недопустимая категория")
)