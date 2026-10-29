package social

import (
	"errors"

	"github.com/go-chi/chi/v5"
)

var errBadToken = errors.New("bad token")

func newTestRouter(h *Webhooks) chi.Router {
	r := chi.NewRouter()
	h.Mount(r)
	return r
}
