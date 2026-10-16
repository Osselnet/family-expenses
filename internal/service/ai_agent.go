package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"family-expenses/internal/domain"
	"family-expenses/internal/llm"
)

type AIAgent struct {
	llmClient      llm.Client
	expenseService *ExpenseService
}

func NewAIAgent(llmClient llm.Client, expenseService *ExpenseService) *AIAgent {
	return &AIAgent{
		llmClient:      llmClient,
		expenseService: expenseService,
	}
}

// aiResponse — структура ответа от LLM.
type aiResponse struct {
	Action   string  `json:"action"`
	Amount   float64 `json:"amount,omitempty"`
	Category string  `json:"category,omitempty"`
	Comment  string  `json:"comment,omitempty"`
	SpentAt  string  `json:"spent_at,omitempty"`
	Period   string  `json:"period,omitempty"`
}

// ProcessMessage обрабатывает сообщение пользователя и возвращает ответ.
func (a *AIAgent) ProcessMessage(ctx context.Context, userID int64, text string) (string, error) {
	// Формируем промпт с текущей датой
	today := time.Now().Format("2006-01-02")
	userPrompt := fmt.Sprintf(llm.UserPromptTemplate, today, text)

	slog.Debug("Отправка запроса к LLM", "user_id", userID, "text", text)
	start := time.Now()

	// Запрашиваем ответ у LLM
	rawResponse, err := a.llmClient.Complete(ctx, llm.SystemPrompt, userPrompt)
	if err != nil {
		slog.Error("Ошибка вызова LLM", "user_id", userID, "duration_ms", time.Since(start).Milliseconds(), "error", err)
		return "", fmt.Errorf("LLM call failed: %w", err)
	}
	slog.Debug("Ответ LLM получен", "user_id", userID, "duration_ms", time.Since(start).Milliseconds(), "raw", rawResponse)

	// Очищаем ответ от возможных маркдаун-обёрток
	rawResponse = strings.TrimSpace(rawResponse)
	rawResponse = strings.TrimPrefix(rawResponse, "```json")
	rawResponse = strings.TrimPrefix(rawResponse, "```")
	rawResponse = strings.TrimSuffix(rawResponse, "```")
	rawResponse = strings.TrimSpace(rawResponse)

	// Парсим JSON-ответ
	var resp aiResponse
	if err := json.Unmarshal([]byte(rawResponse), &resp); err != nil {
		slog.Warn("Ошибка парсинга ответа LLM", "user_id", userID, "raw", rawResponse, "error", err)
		return "Не удалось распознать ответ. Напишите, например: «кофе 350», «такси 500», «траты за неделю».", nil
	}

	slog.Info("LLM распознал действие", "user_id", userID, "action", resp.Action)

	// Выполняем действие
	switch resp.Action {
	case "add":
		return a.handleAdd(ctx, userID, resp)
	case "list":
		return a.handleList(ctx, userID, resp)
	case "update":
		return a.handleUpdate(ctx, userID, resp)
	case "delete":
		return a.handleDelete(ctx, userID, resp)
	default:
		slog.Warn("Неизвестное действие от LLM", "user_id", userID, "action", resp.Action)
		return "Не понял запрос. Попробуйте переформулировать.", nil
	}
}

func (a *AIAgent) handleAdd(ctx context.Context, userID int64, resp aiResponse) (string, error) {
	if resp.Amount <= 0 {
		return "Не удалось определить сумму. Напишите, например: «кофе 350»", nil
	}

	category := domain.Category(resp.Category)
	if !category.Valid() {
		category = domain.CategoryOther
	}

	spentAt := time.Now()
	if resp.SpentAt != "" && resp.SpentAt != "null" {
		parsed, err := time.Parse("2006-01-02", resp.SpentAt)
		if err == nil {
			spentAt = parsed
		}
	}

	input := domain.ExpenseInput{
		Amount:   resp.Amount,
		Category: category,
		SpentAt:  spentAt,
		Comment:  resp.Comment,
	}

	expense, err := a.expenseService.Add(ctx, userID, input)
	if err != nil {
		slog.Error("Ошибка добавления траты через бота", "user_id", userID, "error", err)
		return "", fmt.Errorf("add expense: %w", err)
	}

	slog.Info("Трата добавлена через бота", "expense_id", expense.ID, "user_id", userID, "amount", expense.Amount, "category", expense.Category)
	return "✅ Записал: " + FormatExpense(expense), nil
}

func (a *AIAgent) handleList(ctx context.Context, userID int64, resp aiResponse) (string, error) {
	filter := domain.ExpenseFilter{
		UserID: &userID,
		Limit:  50,
	}

	if resp.Category != "" && resp.Category != "null" {
		cat := domain.Category(resp.Category)
		filter.Category = &cat
	}

	// Определяем период
	now := time.Now()
	switch resp.Period {
	case "сегодня":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		filter.DateFrom = &start
	case "неделя":
		start := now.AddDate(0, 0, -7)
		filter.DateFrom = &start
	case "месяц":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		filter.DateFrom = &start
	}

	expenses, err := a.expenseService.List(ctx, filter)
	if err != nil {
		slog.Error("Ошибка получения списка трат", "user_id", userID, "error", err)
		return "", fmt.Errorf("list expenses: %w", err)
	}

	return FormatExpenseList(expenses), nil
}

func (a *AIAgent) handleUpdate(ctx context.Context, userID int64, resp aiResponse) (string, error) {
	// Получаем последнюю трату пользователя
	last, err := a.expenseService.GetLast(ctx, userID)
	if err != nil {
		if err == domain.ErrNotFound {
			slog.Info("Попытка обновления без трат", "user_id", userID)
			return "У вас пока нет трат для изменения.", nil
		}
		slog.Error("Ошибка получения последней траты", "user_id", userID, "error", err)
		return "", fmt.Errorf("get last expense: %w", err)
	}

	// Обновляем поля, если они указаны
	input := domain.ExpenseInput{
		Amount:   last.Amount,
		Category: last.Category,
		SpentAt:  last.SpentAt,
		Comment:  last.Comment,
	}

	if resp.Amount > 0 {
		input.Amount = resp.Amount
	}
	if resp.Comment != "" && resp.Comment != "null" {
		input.Comment = resp.Comment
	}

	updated, err := a.expenseService.Update(ctx, userID, last.ID, input)
	if err != nil {
		slog.Error("Ошибка обновления траты", "user_id", userID, "expense_id", last.ID, "error", err)
		return "", fmt.Errorf("update expense: %w", err)
	}

	slog.Info("Трата обновлена через бота", "expense_id", updated.ID, "user_id", userID)
	return "✏️ Обновил: " + FormatExpense(updated), nil
}

func (a *AIAgent) handleDelete(ctx context.Context, userID int64, resp aiResponse) (string, error) {
	// Ищем последнюю трату (или по комментарию)
	last, err := a.expenseService.GetLast(ctx, userID)
	if err != nil {
		if err == domain.ErrNotFound {
			slog.Info("Попытка удаления без трат", "user_id", userID)
			return "У вас пока нет трат для удаления.", nil
		}
		slog.Error("Ошибка получения последней траты для удаления", "user_id", userID, "error", err)
		return "", fmt.Errorf("get last expense: %w", err)
	}

	if err := a.expenseService.Delete(ctx, userID, last.ID); err != nil {
		slog.Error("Ошибка удаления траты", "user_id", userID, "expense_id", last.ID, "error", err)
		return "", fmt.Errorf("delete expense: %w", err)
	}

	slog.Info("Трата удалена через бота", "expense_id", last.ID, "user_id", userID)
	return "🗑 Удалил: " + FormatExpense(last), nil
}