package api

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/dselans/blastbeat-api/services/release"
)

func (a *API) releasesHandler(rw http.ResponseWriter, r *http.Request) {
	logger := a.log.With(zap.String("method", "releasesHandler"))
	logger.Info("handling /api/releases request", zap.String("remoteAddr", r.RemoteAddr))

	// Parse query parameters
	filters := &release.ReleaseFilters{}

	// dateFrom
	if dateFromStr := r.URL.Query().Get("dateFrom"); dateFromStr != "" {
		dateFrom, err := time.Parse("2006-01-02", dateFromStr)
		if err != nil {
			a.writeError(rw, http.StatusBadRequest, "Invalid dateFrom parameter")
			return
		}
		filters.DateFrom = &dateFrom
	}

	if dateToStr := r.URL.Query().Get("dateTo"); dateToStr != "" {
		dateTo, err := time.Parse("2006-01-02", dateToStr)
		if err != nil {
			a.writeError(rw, http.StatusBadRequest, "Invalid dateTo parameter")
			return
		}
		filters.DateTo = &dateTo
	}

	// includedGenres
	includedGenres := r.URL.Query()["includedGenres"]
	if len(includedGenres) > 0 {
		filters.IncludedGenres = includedGenres
	}

	// excludedGenres
	excludedGenres := r.URL.Query()["excludedGenres"]
	if len(excludedGenres) > 0 {
		filters.ExcludedGenres = excludedGenres
	}

	// excludedKeywords
	excludedKeywords := r.URL.Query()["excludedKeywords"]
	if len(excludedKeywords) > 0 {
		filters.ExcludedKeywords = excludedKeywords
	}

	// followerRange
	if followerRange := r.URL.Query().Get("followerRange"); followerRange != "" {
		filters.FollowerRange = followerRange
	}

	// Validate that if one date is provided, both should be provided
	if (filters.DateFrom != nil && filters.DateTo == nil) || (filters.DateFrom == nil && filters.DateTo != nil) {
		a.writeError(rw, http.StatusBadRequest, "Both dateFrom and dateTo must be provided together")
		return
	}

	// Fetch releases from service
	releases, err := a.deps.ReleaseService.GetReleases(r.Context(), filters)
	if err != nil {
		logger.Error("Failed to fetch releases", zap.Error(err))
		a.writeError(rw, http.StatusInternalServerError, "Failed to fetch releases")
		return
	}

	// Write response
	rw.Header().Set("Content-Type", "application/json; charset=UTF-8")
	rw.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(rw).Encode(releases); err != nil {
		logger.Error("Failed to encode releases response", zap.Error(err))
	}
}

func (a *API) writeError(rw http.ResponseWriter, statusCode int, message string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(statusCode)

	errorResponse := map[string]string{
		"error": message,
	}

	if err := json.NewEncoder(rw).Encode(errorResponse); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
	}
}
