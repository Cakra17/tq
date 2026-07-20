package middleware

import (
	"net/http"

	"github.com/cakra17/tq/internal/models"
)

const (
	HeaderIdempotencyKey      = "Idempotancy-Key"
	HeaderIdempotencyReplayed = "Idempotancy-Replayed"
)

type Store interface {
	Get(key string) (*models.Response, error)
	Set(key string, res *models.Response) error
	Delete(key string) error
	SetProcessingKey(key string) (bool, error)
}

type Idempotancy struct {
	store Store
}

func NewIdempotancy(store Store) *Idempotancy {
	return &Idempotancy{
		store: store,
	}
}

func (m *Idempotancy) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get(HeaderIdempotencyKey)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		cached, err := m.store.Get(key)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if cached != nil && cached.StatusCode != 0 {
			m.replayResponse(w, cached)
			return
		}

		isValid, err := m.store.SetProcessingKey(key)
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		if !isValid {
			models.SendResponse(w, http.StatusConflict, models.Response{
				Message: "Request with this idempotency key is already being processed",
			})
			return
		}

		next.ServeHTTP(w, r)

		resp := &models.Response{
			StatusCode: http.StatusOK,
			Data:       r.Body,
		}

		m.store.Set(key, resp)
	})
}

func (m *Idempotancy) replayResponse(w http.ResponseWriter, resp *models.Response) {
	for key, value := range resp.Header {
		w.Header().Set(key, value)
	}
	w.Header().Set(HeaderIdempotencyReplayed, "true")
	models.SendResponse(w, resp.StatusCode, *resp)
}
