package llm

import "context"

// Client — интерфейс для работы с LLM-провайдером.
// Реализации: OpenAI, Anthropic, локальные модели.
type Client interface {
	// Complete отправляет промпт и получает структурированный ответ.
	Complete(ctx context.Context, systemPrompt, userMessage string) (string, error)
}