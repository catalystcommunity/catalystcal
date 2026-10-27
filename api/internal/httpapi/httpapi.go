// Package httpapi assembles the public HTTP surface of catalystcal:
//
//	POST /csil/v1/rpc, POST /api/csil/v1/rpc   the CSIL-RPC carrier
//	GET  /api/v1/auth/start                    302 to the LinkKeys IDP
//	GET  /api/health                           liveness for the API port
//	GET  /ics/{token}, GET /api/ics/{token}    the public iCalendar feed
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/services"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
)

// Handler builds the API handler. devAuth registers DevAuthService.
func Handler(deps *services.Deps, devAuth bool) http.Handler {
	d := csilrpc.New(deps.Secret)
	services.Register(d, deps, devAuth)

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+csilrpc.RPCMountPath, d.ServeRPC)
	mux.HandleFunc("POST /api"+csilrpc.RPCMountPath, d.ServeRPC)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("GET /api/v1/auth/start", authStart(deps))
	feed := feedHandler(deps)
	mux.Handle("GET /ics/{token}", feed)
	mux.Handle("GET /api/ics/{token}", feed)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	return c.Handler(mux)
}

// authStart signs a LinkKeys auth request bound to the callback URL and a
// self-verifying login nonce, then sends the browser to the IDP.
func authStart(deps *services.Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := deps.Cfg
		if deps.PKI == nil || cfg == nil || cfg.LinkkeysIDPURL == "" || cfg.CallbackURL == "" {
			http.Error(w, "browser login is not configured on this server", http.StatusServiceUnavailable)
			return
		}
		nonce := auth.MintNonce(deps.Secret)
		signed, err := deps.PKI.SignRequest(cfg.CallbackURL, nonce)
		if err != nil {
			log.WithError(err).Error("auth/start: RP sign-request failed")
			http.Error(w, "could not reach identity service", http.StatusBadGateway)
			return
		}
		q := url.Values{}
		q.Set("signed_request", signed)
		http.Redirect(w, r, cfg.LinkkeysIDPURL+"/auth/authorize?"+q.Encode(), http.StatusFound)
	})
}

func feedHandler(deps *services.Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		doc, err := deps.Feed(r.Context(), r.PathValue("token"))
		if errors.Is(err, services.ErrFeedNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			log.WithError(err).Error("feed: render failed")
			http.Error(w, "could not render the feed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "private, max-age=300")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(doc)
	})
}
