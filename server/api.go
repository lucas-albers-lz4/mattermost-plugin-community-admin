package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (p *Plugin) initRouter() *mux.Router {
	router := mux.NewRouter()
	router.Use(p.mattermostAuthorizationRequired)

	api := router.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/me", p.handleMe).Methods(http.MethodGet)
	api.HandleFunc("/users", p.handleListUsers).Methods(http.MethodGet)
	api.HandleFunc("/users", p.handleCreateUser).Methods(http.MethodPost)
	api.HandleFunc("/users/batch", p.handleBatchImport).Methods(http.MethodPost)
	api.HandleFunc("/users/{id}", p.handlePatchUser).Methods(http.MethodPatch)
	api.HandleFunc("/users/{id}/reset-password", p.handleResetPassword).Methods(http.MethodPost)
	api.HandleFunc("/users/{id}/activate", p.handleActivate).Methods(http.MethodPost)
	api.HandleFunc("/users/{id}/deactivate", p.handleDeactivate).Methods(http.MethodPost)
	api.HandleFunc("/users/{id}/teams/{teamId}", p.handleAddTeamMember).Methods(http.MethodPut)
	api.HandleFunc("/users/{id}/teams/{teamId}", p.handleRemoveTeamMember).Methods(http.MethodDelete)
	api.HandleFunc("/users/{id}/channels/{channelId}", p.handleAddChannelMember).Methods(http.MethodPut)
	api.HandleFunc("/users/{id}/channels/{channelId}", p.handleRemoveChannelMember).Methods(http.MethodDelete)
	api.HandleFunc("/audit", p.handleAudit).Methods(http.MethodGet)
	api.HandleFunc("/resolve-scope", p.handleResolveScope).Methods(http.MethodPost)

	return router
}

func (p *Plugin) mattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-Id")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
