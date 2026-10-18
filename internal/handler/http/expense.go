package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"family-expenses/internal/domain"
	"family-expenses/internal/service"
)

type ExpenseHandler struct {
	service *service.ExpenseService
}

func NewExpenseHandler(service *service.ExpenseService) *ExpenseHandler {
	return &ExpenseHandler{service: service}
}

// CreateRequest — запрос на создание/обновление траты.
type CreateRequest struct {
	UserID   int64   `json:"user_id"`
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	SpentAt  string  `json:"spent_at"`
	Comment  string  `json:"comment"`
}

func (h *ExpenseHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Ограничение размера тела запроса (1 MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("Неверный формат запроса на создание", "error", err)
		writeError(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	input := domain.ExpenseInput{
		Amount:   req.Amount,
		Category: domain.Category(req.Category),
		Comment:  req.Comment,
	}

	if req.SpentAt != "" {
		t, err := parseTime(req.SpentAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Неверный формат даты: %s", req.SpentAt))
			return
		}
		input.SpentAt = t
	}

	// Валидация входных данных
	if err := input.Validate(); err != nil {
		slog.Warn("Ошибка валидации при создании", "user_id", req.UserID, "error", err)
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	expense, err := h.service.Add(r.Context(), req.UserID, input)
	if err != nil {
		slog.Error("Ошибка создания траты", "user_id", req.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	slog.Info("Трата создана через API", "expense_id", expense.ID, "user_id", req.UserID, "amount", expense.Amount, "category", expense.Category)
	writeJSON(w, http.StatusCreated, expense)
}

func (h *ExpenseHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	expense, err := h.service.GetByIDAll(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Трата не найдена")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, expense)
}

func (h *ExpenseHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Ограничение размера тела запроса (1 MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	input := domain.ExpenseInput{
		Amount:   req.Amount,
		Category: domain.Category(req.Category),
		Comment:  req.Comment,
	}

	if req.SpentAt != "" {
		t, err := parseTime(req.SpentAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Неверный формат даты: %s", req.SpentAt))
			return
		}
		input.SpentAt = t
	}

	if err := input.Validate(); err != nil {
		slog.Warn("Ошибка валидации при обновлении", "id", id, "error", err)
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	expense, err := h.service.UpdateAll(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Трата не найдена")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, expense)
}

func (h *ExpenseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.service.DeleteAll(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Трата не найдена")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAll — для веб-панели: все траты семьи с фильтрами.
func (h *ExpenseHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := domain.ExpenseFilter{}

	if userID := query.Get("user_id"); userID != "" {
		id, err := strconv.ParseInt(userID, 10, 64)
		if err == nil {
			filter.UserID = &id
		}
	}
	if category := query.Get("category"); category != "" {
		cat := domain.Category(category)
		filter.Category = &cat
	}
	if dateFrom := query.Get("date_from"); dateFrom != "" {
		t, err := parseTime(dateFrom)
		if err == nil {
			filter.DateFrom = &t
		}
	}
	if dateTo := query.Get("date_to"); dateTo != "" {
		t, err := parseTime(dateTo)
		if err == nil {
			filter.DateTo = &t
		}
	}
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	expenses, err := h.service.ListAll(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, expenses)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}
