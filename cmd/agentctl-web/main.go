package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Clawdlinux/agentic-operator-core/pkg/agentctl"
	"github.com/Clawdlinux/agentic-operator-core/pkg/webtheme"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var version = "dev"

func main() {
	var (
		addr       string
		kubeconfig string
		demoMode   bool
		decisions  decisionSourceConfig
	)
	flag.StringVar(&addr, "addr", ":8090", "HTTP listen address")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (empty = in-cluster)")
	flag.BoolVar(&demoMode, "demo", false, "Run the booth demo UI without Kubernetes")
	flag.StringVar(&decisions.ExportDir, "export-dir", "", "serve the decision pages offline from a receipt export directory (no cluster)")
	flag.StringVar(&decisions.WriterURL, "writer-url", "", "receipt-writer base URL for the decision pages")
	flag.StringVar(&decisions.TokenFile, "writer-token-file", "", "file holding the receipt-writer bearer token")
	flag.StringVar(&decisions.TrustRoot, "trust-root", "", "trust file pinned out of band; without it the chain shows as consistent only")
	flag.DurationVar(&decisions.HTTPTimeout, "writer-timeout", 5*time.Second, "HTTP timeout for the receipt-writer")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	addrSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrSet = true
		}
	})

	var decisionUI *decisionPages
	if decisions.enabled() {
		loader, err := newDecisionLoader(decisions)
		if err != nil {
			slog.Error("decision pages", "error", err)
			os.Exit(2)
		}
		if decisionUI, err = newDecisionPages(loader, TemplatesFS()); err != nil {
			slog.Error("decision pages", "error", err)
			os.Exit(1)
		}
		if decisions.TrustRoot == "" {
			slog.Warn("no --trust-root: the chain will show as consistent only, not authenticated")
		}
	}

	// An export directory alone is the offline path: no cluster, no auth, and it
	// binds to localhost unless --addr says otherwise.
	if decisions.ExportDir != "" && !demoMode {
		if !addrSet {
			addr = "127.0.0.1:8090"
		}
		slog.Info("starting agentctl-web offline decision pages", "version", version, "addr", addr, "export", decisions.ExportDir)
		runHTTPServer(addr, offlineHandler(decisionUI))
		return
	}

	slog.Info("starting agentctl-web", "version", version, "addr", addr)

	if demoMode {
		var demoClient *agentctl.Client
		demoKubeconfig := kubeconfig
		if demoKubeconfig == "" {
			if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
				demoKubeconfig = clientcmd.RecommendedHomeFile
			}
		}
		if demoKubeconfig != "" {
			cfg, err := clientcmd.BuildConfigFromFlags("", demoKubeconfig)
			if err != nil {
				slog.Warn("demo mode running without cluster", "error", err)
			} else if client, err := agentctl.NewClient(cfg); err != nil {
				slog.Warn("demo mode failed to create cluster client", "error", err)
			} else {
				demoClient = client
			}
		}

		srv, err := NewServer(demoClient, nil, TemplatesFS())
		if err != nil {
			slog.Error("failed to create demo server", "error", err)
			os.Exit(1)
		}

		runHTTPServer(addr, demoHandler(srv))
		return
	}

	// Build K8s config
	var cfg *rest.Config
	var err error
	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		slog.Error("failed to build k8s config", "error", err)
		os.Exit(1)
	}

	// Create agentctl client
	client, err := agentctl.NewClient(cfg)
	if err != nil {
		slog.Error("failed to create agentctl client", "error", err)
		os.Exit(1)
	}

	// Create K8s client for auth
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		slog.Error("failed to create kubernetes client", "error", err)
		os.Exit(1)
	}

	authn := NewTokenAuthenticator(kubeClient)
	authz := NewAuthorizer(kubeClient)

	// Create server
	srv, err := NewServer(client, authz, TemplatesFS())
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	// Routes
	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS()))))
	mux.Handle("GET /theme/", http.StripPrefix("/theme/", http.FileServer(http.FS(webtheme.FS()))))

	// Health endpoints
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleHealthz)

	// Auth endpoints
	mux.HandleFunc("GET /auth/login", srv.handleLogin)
	mux.HandleFunc("POST /auth/login", srv.handleLogin)
	mux.HandleFunc("POST /auth/logout", srv.handleLogout)

	// Dashboard
	mux.HandleFunc("GET /", srv.handleDashboard)
	mux.HandleFunc("GET /{$}", srv.handleDashboard)
	mux.HandleFunc("GET /demo", srv.handleDemo)

	// Workloads
	mux.HandleFunc("GET /workloads", srv.handleWorkloads)
	mux.HandleFunc("GET /workloads/{ns}/{name}", srv.handleDescribe)
	mux.HandleFunc("POST /workloads/{ns}/{name}/approve", srv.handleApprove)
	mux.HandleFunc("POST /workloads/{ns}/{name}/reject", srv.handleReject)

	// Cost & Status
	mux.HandleFunc("GET /cost", srv.handleCost)
	mux.HandleFunc("GET /status", srv.handleStatus)

	if decisionUI != nil {
		decisionUI.register(mux)
		srv.decisionsOn = true
	}

	// Middleware chain
	var handler http.Handler = mux
	handler = CSRFMiddleware(handler)
	handler = AuthMiddleware(authn)(handler)
	handler = AuditMiddleware(handler)
	handler = RequestIDMiddleware(handler)

	runHTTPServer(addr, handler)
}

// offlineHandler serves only the decision pages. It needs no cluster.
func offlineHandler(d *decisionPages) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS()))))
	mux.Handle("GET /theme/", http.StripPrefix("/theme/", http.FileServer(http.FS(webtheme.FS()))))
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleHealthz)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/decisions", http.StatusSeeOther)
	})
	d.register(mux)
	return RequestIDMiddleware(AuditMiddleware(mux))
}

func demoHandler(srv *Server) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS()))))
	mux.Handle("GET /theme/", http.StripPrefix("/theme/", http.FileServer(http.FS(webtheme.FS()))))
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleHealthz)
	mux.HandleFunc("GET /", srv.handleDemo)
	mux.HandleFunc("GET /{$}", srv.handleDemo)
	mux.HandleFunc("GET /demo", srv.handleDemo)
	return RequestIDMiddleware(AuditMiddleware(mux))
}

func runHTTPServer(addr string, handler http.Handler) {
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	fmt.Println("agentctl-web stopped")
}
