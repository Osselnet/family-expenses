package http

import (
	"log/slog"
	"net/http"
	"time"

	"family-expenses/internal/service"
)

// loggingMiddleware логирует все HTTP-запросы (метод, путь, статус, длительность).
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)

		slog.Info("HTTP запрос",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func NewRouter(expenseService *service.ExpenseService, webFS http.FileSystem) http.Handler {
	mux := http.NewServeMux()

	expenseHandler := NewExpenseHandler(expenseService)

	// REST API для веб-панели (все траты семьи, без авторизации)
	mux.HandleFunc("GET /api/expenses", expenseHandler.ListAll)
	mux.HandleFunc("POST /api/expenses", expenseHandler.Create)
	mux.HandleFunc("GET /api/expenses/{id}", expenseHandler.Get)
	mux.HandleFunc("PUT /api/expenses/{id}", expenseHandler.Update)
	mux.HandleFunc("DELETE /api/expenses/{id}", expenseHandler.Delete)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Веб-панель (catch-all для не-API путей)
	if webFS != nil {
		mux.Handle("/", http.FileServer(webFS))
	}

	return loggingMiddleware(mux)
}