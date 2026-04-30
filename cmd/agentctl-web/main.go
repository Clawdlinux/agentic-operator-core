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

	"github.com/shreyansh/agentic-operator/pkg/agentctl"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var version = "dev"

func main() {
	var (
		addr       string
		kubeconfig string
	)
	flag.StringVar(&addr, "addr", ":8090", "HTTP listen address")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (empty = in-cluster)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("starting agentctl-web", "version", version, "addr", addr)

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

	// Workloads
	mux.HandleFunc("GET /workloads", srv.handleWorkloads)
	mux.HandleFunc("GET /workloads/{ns}/{name}", srv.handleDescribe)
	mux.HandleFunc("POST /workloads/{ns}/{name}/approve", srv.handleApprove)
	mux.HandleFunc("POST /workloads/{ns}/{name}/reject", srv.handleReject)

	// Cost & Status
	mux.HandleFunc("GET /cost", srv.handleCost)
	mux.HandleFunc("GET /status", srv.handleStatus)

	// Middleware chain
	var handler http.Handler = mux
	handler = CSRFMiddleware(handler)
	handler = AuthMiddleware(authn)(handler)
	handler = AuditMiddleware(handler)
	handler = RequestIDMiddleware(handler)

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
