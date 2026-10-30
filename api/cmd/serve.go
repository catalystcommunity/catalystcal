package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/actas"
	"github.com/catalystcommunity/catalystcal/api/internal/config"
	"github.com/catalystcommunity/catalystcal/api/internal/httpapi"
	"github.com/catalystcommunity/catalystcal/api/internal/linkkeys"
	"github.com/catalystcommunity/catalystcal/api/internal/services"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	"github.com/catalystcommunity/catalystcal/api/internal/subscription"
	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	log "github.com/sirupsen/logrus"
)

// readiness is the /readyz state: a cause until the boot finishes, then a
// live database ping.
type readiness struct {
	cause atomic.Value // string; "" when the boot finished
	db    atomic.Pointer[sql.DB]
}

func (r *readiness) set(cause string) { r.cause.Store(cause) }

func (r *readiness) handler(w http.ResponseWriter, req *http.Request) {
	if c, _ := r.cause.Load().(string); c != "" {
		http.Error(w, c, http.StatusServiceUnavailable)
		return
	}
	db := r.db.Load()
	if db == nil {
		http.Error(w, "database not open", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		http.Error(w, "database ping failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("ready\n"))
}

// runServe follows the boot order that keeps probes honest: the ops
// listener first, then migrations, then the store, then the API listener.
func runServe(version string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signalContext()
	defer cancel()

	ready := &readiness{}
	ready.set("starting")
	opsMux := http.NewServeMux()
	opsMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	opsMux.HandleFunc("GET /readyz", ready.handler)
	opsSrv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.OpsPort), Handler: opsMux, ReadHeaderTimeout: 5 * time.Second}
	opsLn, err := net.Listen("tcp", opsSrv.Addr)
	if err != nil {
		return fmt.Errorf("ops listener: %w", err)
	}
	go func() {
		if err := opsSrv.Serve(opsLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Error("ops server stopped")
		}
	}()
	defer func() {
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		_ = opsSrv.Shutdown(sctx)
	}()
	log.WithFields(log.Fields{"version": version, "env": cfg.Env, "ops_port": cfg.OpsPort}).Info("catalystcal starting")

	ready.set("opening database")
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	ready.set("applying migrations")
	if err := store.Migrate(ctx, db); err != nil {
		if ctx.Err() != nil {
			return errShutdown
		}
		return fmt.Errorf("migrate: %w", err)
	}
	if err := waitForMigrations(ctx, db, ready); err != nil {
		return err
	}
	st := store.New(db)
	ready.db.Store(db)

	deps, err := buildDeps(ctx, cfg, st)
	if err != nil {
		return err
	}
	devAuth := cfg.DevAuthAllowed()
	if cfg.DevAuthEnabled && !devAuth {
		log.WithField("env", cfg.Env).Warn("CATALYSTCAL_DEV_AUTH_ENABLED ignored: CATALYSTCAL_ENV is not dev or nonprod")
	}
	if devAuth {
		log.WithField("env", cfg.Env).Warn("DEV-AUTH ENABLED: DevAuthService signs people in without LinkKeys")
	}

	workers, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	go every(workers, cfg.SubscriptionRefresh, "subscriptions", func(c context.Context) {
		deps.SyncDueSubscriptions(c, cfg.SubscriptionRefresh)
	})
	if deps.ActAs != nil {
		go every(workers, 5*time.Minute, "act-as revocations", func(c context.Context) {
			if err := deps.RefreshRevocations(c); err != nil {
				log.WithError(err).Warn("act-as revocation refresh incomplete")
			}
		})
	}

	apiSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           httpapi.Handler(deps, devAuth),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	apiLn, err := net.Listen("tcp", apiSrv.Addr)
	if err != nil {
		return fmt.Errorf("api listener: %w", err)
	}
	ready.set("")
	log.WithField("api_port", cfg.APIPort).Info("catalystcal ready")
	errCh := make(chan error, 1)
	go func() { errCh <- apiSrv.Serve(apiLn) }()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("api server: %w", err)
		}
	}
	ready.set("shutting down")
	stopWorkers()
	sctx, scancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer scancel()
	if err := apiSrv.Shutdown(sctx); err != nil {
		log.WithError(err).Warn("api shutdown")
	}
	return nil
}

// waitForMigrations polls until no migration is pending. An operator who
// runs migrations as a separate step then gets a waiting pod, not a crash
// loop. A signal ends the wait with errShutdown.
func waitForMigrations(ctx context.Context, db *sql.DB, ready *readiness) error {
	for {
		pending, err := store.Pending(ctx, db)
		if err == nil && !pending {
			return nil
		}
		if err != nil {
			ready.set("checking migrations: " + err.Error())
		} else {
			ready.set("waiting for migrations")
		}
		select {
		case <-ctx.Done():
			return errShutdown
		case <-time.After(2 * time.Second):
		}
	}
}

func every(ctx context.Context, interval time.Duration, name string, fn func(context.Context)) {
	log.WithFields(log.Fields{"worker": name, "interval": interval}).Info("worker started")
	t := time.NewTicker(interval)
	defer t.Stop()
	fn(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn(ctx)
		}
	}
}

func buildDeps(ctx context.Context, cfg *config.Config, st *store.Store) (*services.Deps, error) {
	deps := &services.Deps{
		Store:   st,
		Cfg:     cfg,
		Secret:  []byte(cfg.JWTSecret),
		Fetcher: &subscription.Fetcher{AllowPrivate: cfg.SubscriptionAllowPrivate},
		Revoked: services.NewRevokedGrants(),
	}
	if pki := buildPKI(cfg); pki != nil {
		deps.PKI = pki
	} else {
		log.Warn("LinkKeys RP not configured: AuthService.Complete answers 503")
	}
	if cfg.ActAsAppKeysFile != "" {
		p, err := buildActAs(cfg)
		if err != nil {
			return nil, err
		}
		deps.ActAs = p
		log.Info("act-as audience enabled")
	}
	if err := deps.LoadRevoked(ctx); err != nil {
		return nil, fmt.Errorf("load revoked act-as grants: %w", err)
	}
	return deps, nil
}

// buildPKI makes the LinkKeys RP client (CSIL-RPC over TCP+TLS), or nil
// when it is not configured.
func buildPKI(cfg *config.Config) services.PKIClient {
	if cfg.LinkkeysIDPDomain == "" && cfg.LinkkeysTCPAddr == "" {
		return nil
	}
	c, err := linkkeys.NewTCPClient(linkkeys.TCPConfig{
		Domain:       cfg.LinkkeysIDPDomain,
		APIKey:       cfg.LinkkeysPKIAPIKey,
		Addr:         cfg.LinkkeysTCPAddr,
		Fingerprints: cfg.LinkkeysTCPFingerprints,
		Insecure:     cfg.LinkkeysTCPAllowInsecure,
	})
	if err != nil {
		log.WithError(err).Error("LinkKeys RP client init failed; Complete answers 503")
		return nil
	}
	return c
}

// buildActAs makes the SDK-backed act-as audience. A configured key file
// with a bad RP connection setting stops the boot.
func buildActAs(cfg *config.Config) (actas.Provider, error) {
	keys, err := actas.LoadAppKeys(cfg.ActAsAppKeysFile)
	if err != nil {
		return nil, err
	}
	signers, err := keys.Signers()
	if err != nil {
		return nil, err
	}
	t, err := regularrp.NewPinnedRpcTransport(regularrp.PinnedRpcTransportOptions{
		TCPAddress:   cfg.LinkkeysTCPAddr,
		Fingerprints: cfg.LinkkeysTCPFingerprints,
		APIKey:       cfg.LinkkeysPKIAPIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("act-as needs CATALYSTCAL_LINKKEYS_TCP_ADDR, CATALYSTCAL_LINKKEYS_TCP_FINGERPRINTS, and CATALYSTCAL_LINKKEYS_PKI_API_KEY: %w", err)
	}
	return &actas.Verifier{
		Own:        keys.Ref(),
		InstanceID: keys.InstanceID,
		Signers:    signers,
		Keys:       actas.NewRPKeySource(t),
		MaxAge:     cfg.ActAsMaxPresentationAge,
	}, nil
}
