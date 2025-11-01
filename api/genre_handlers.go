package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

type GenreResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (a *API) genresHandler(rw http.ResponseWriter, r *http.Request) {
	logger := a.log.With(zap.String("method", "genresHandler"))
	logger.Info("handling /api/genres request", zap.String("remoteAddr", r.RemoteAddr))

	// Fetch genres directly from database
	dbGenres, err := a.deps.DBBackend.ListGenres(r.Context())
	if err != nil {
		logger.Error("Failed to fetch genres", zap.Error(err))
		a.writeError(rw, http.StatusInternalServerError, "Failed to fetch genres")
		return
	}

	// Convert to response format
	genres := make([]GenreResponse, 0, len(dbGenres))

	for _, dbGenre := range dbGenres {
		genres = append(genres, GenreResponse{
			ID:   dbGenre.ID.String(),
			Name: dbGenre.Name,
			Slug: dbGenre.Slug,
		})
	}

	// Write response
	rw.Header().Set("Content-Type", "application/json; charset=UTF-8")
	rw.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(rw).Encode(genres); err != nil {
		logger.Error("Failed to encode genres response", zap.Error(err))
	}
}
