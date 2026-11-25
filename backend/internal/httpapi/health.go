package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status   string    `json:"status"`
	Database string    `json:"database"`
	Version  string    `json:"version"`
	Time     time.Time `json:"time"`
}

// newHealthHandler 同时作为存活与就绪探针：数据库不可达时返回 503。
func newHealthHandler(pool *pgxpool.Pool, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		response := healthResponse{
			Status:   "ok",
			Database: "ok",
			Version:  Version,
			Time:     time.Now().UTC(),
		}
		status := http.StatusOK

		if err := pool.Ping(ctx); err != nil {
			logger.Warn("health check: database unreachable", "error", err)
			response.Status = "degraded"
			response.Database = "down"
			status = http.StatusServiceUnavailable
		}

		writeJSON(w, status, response)
	}
}
