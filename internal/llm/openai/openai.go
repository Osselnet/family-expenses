package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"family-expenses/internal/llm"
)

// Client — реализация llm.Client для OpenAI-совместимого API.
type Client struct {
	baseURL     string
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
	httpClient  *http.Client
}

func NewClient(baseURL, apiKey, model string, temperature float64, maxTokens int, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 120 * time.Second // локальные LLM могут думать долго
	}
	return &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []message     `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (c *Client) Complete(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	start := time.Now()

	reqBody := chatRequest{
		Model:       c.model,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("LLM HTTP запрос", "url", url, "model", c.model, "duration_ms", time.Since(start).Milliseconds(), "error", err)
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("LLM HTTP ошибка", "url", url, "status", resp.StatusCode, "duration_ms", time.Since(start).Milliseconds(), "body", string(respBytes))
		return "", fmt.Errorf("LLM API error: status %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		slog.Error("LLM ошибка парсинга ответа", "url", url, "duration_ms", time.Since(start).Milliseconds(), "error", err)
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		slog.Error("LLM ошибка API", "url", url, "message", chatResp.Error.Message, "duration_ms", time.Since(start).Milliseconds())
		return "", fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned empty response")
	}

	slog.Debug("LLM ответ получен", "model", c.model, "response_len", len(chatResp.Choices[0].Message.Content), "duration_ms", time.Since(start).Milliseconds())
	return chatResp.Choices[0].Message.Content, nil
}

// Убеждаемся, что Client реализует интерфейс.
var _ llm.Client = (*Client)(nil)