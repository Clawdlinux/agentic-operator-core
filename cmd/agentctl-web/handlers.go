package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Clawdlinux/agentic-operator-core/pkg/agentctl"
)

// Server holds the web server dependencies.
type Server struct {
	client             *agentctl.Client
	authz              *Authorizer
	tmpl               *template.Template
	demoPollTimeout    time.Duration
	demoPodSource      func(context.Context) ([]agentctl.RuntimePodRow, error)
	demoWorkloadSource func(context.Context) ([]agentctl.WorkloadRow, error)
}

type demoPageData struct {
	CSRFToken             string
	Demo                  bool
	Live                  bool
	RuntimePodsLive       bool
	WorkloadsLive         bool
	WorkloadAggregateLive bool
	ObservedGVisorPod     bool
	RuntimePods           []agentctl.RuntimePodRow
	Workloads             []agentctl.WorkloadRow
	TotalWorkloads        int
	TotalCostToday        float64
	User                  *UserInfo
}

// NewServer creates a Server with parsed templates.
func NewServer(client *agentctl.Client, authz *Authorizer, tmplFS fs.FS) (*Server, error) {
	funcMap := template.FuncMap{
		"lower":    strings.ToLower,
		"safeText": agentctl.SafeText,
		"fmtCost": func(f float64) string {
			return fmt.Sprintf("$%.4f", f)
		},
		"fmtTokens": func(i int64) string {
			return fmt.Sprintf("%d", i)
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(tmplFS, "*.html", "partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	server := &Server{
		client:          client,
		authz:           authz,
		tmpl:            tmpl,
		demoPollTimeout: 2 * time.Second,
	}
	if client != nil {
		if client.Kube != nil {
			server.demoPodSource = func(ctx context.Context) ([]agentctl.RuntimePodRow, error) {
				return client.ListRuntimePods(ctx, "")
			}
		}
		if client.Dynamic != nil {
			server.demoWorkloadSource = func(ctx context.Context) ([]agentctl.WorkloadRow, error) {
				return client.ListWorkloads(ctx, "")
			}
		}
	}

	return server, nil
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(userContextKey).(*UserInfo)
	if !ok || user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	csrfToken, _ := r.Context().Value(csrfContextKey).(string)

	status, err := s.client.ClusterStatus(r.Context(), "")
	if err != nil {
		slog.Error("cluster status", "error", err)
		http.Error(w, "Failed to fetch cluster status", http.StatusInternalServerError)
		return
	}

	workloads, err := s.client.ListWorkloads(r.Context(), "")
	if err != nil {
		slog.Error("list workloads", "error", err)
		http.Error(w, "Failed to list workloads", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"User":      user,
		"CSRFToken": csrfToken,
		"Status":    status,
		"Workloads": workloads,
	}

	if err := s.tmpl.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		slog.Error("render dashboard", "error", err)
	}
}

func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	data := demoPageData{
		CSRFToken:   "demo",
		Demo:        true,
		RuntimePods: []agentctl.RuntimePodRow{},
		Workloads:   []agentctl.WorkloadRow{},
		User: &UserInfo{
			Username: "booth-demo",
		},
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.demoPollTimeout)
	defer cancel()

	type demoSourceResult struct {
		source    string
		pods      []agentctl.RuntimePodRow
		workloads []agentctl.WorkloadRow
		err       error
	}

	results := make(chan demoSourceResult, 2)
	remaining := 0
	if s.demoPodSource != nil {
		remaining++
		go func() {
			rows, err := s.demoPodSource(ctx)
			results <- demoSourceResult{source: "runtime-pods", pods: rows, err: err}
		}()
	}
	if s.demoWorkloadSource != nil {
		remaining++
		go func() {
			rows, err := s.demoWorkloadSource(ctx)
			results <- demoSourceResult{source: "workloads", workloads: rows, err: err}
		}()
	}

	for remaining > 0 {
		select {
		case result := <-results:
			remaining--
			applyDemoSourceResult(&data, result.source, result.pods, result.workloads, result.err)
		case <-ctx.Done():
			for remaining > 0 {
				result := <-results
				remaining--
				applyDemoSourceResult(&data, result.source, result.pods, result.workloads, result.err)
			}
		}
	}
	data.Live = data.RuntimePodsLive || data.WorkloadsLive

	if err := s.tmpl.ExecuteTemplate(w, "demo.html", data); err != nil {
		slog.Error("render demo", "error", err)
		http.Error(w, "Failed to render demo", http.StatusInternalServerError)
	}
}

func applyDemoSourceResult(data *demoPageData, source string, pods []agentctl.RuntimePodRow, workloads []agentctl.WorkloadRow, err error) {
	if err != nil {
		logDemoSourceError(source, err)
		return
	}

	switch source {
	case "runtime-pods":
		data.RuntimePods = pods
		data.RuntimePodsLive = true
		data.ObservedGVisorPod = hasGVisorRuntimePod(pods)
	case "workloads":
		data.Workloads = workloads
		data.WorkloadsLive = true
		data.WorkloadAggregateLive = true
		data.TotalWorkloads, data.TotalCostToday = aggregateWorkloads(workloads)
	}
}

func logDemoSourceError(source string, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		slog.Debug("demo source unavailable", "source", source, "error", err)
		return
	}
	slog.Warn("demo source unavailable", "source", source, "error", err)
}

func aggregateWorkloads(rows []agentctl.WorkloadRow) (int, float64) {
	var totalCostToday float64
	for _, row := range rows {
		totalCostToday += row.CostToday
	}
	return len(rows), totalCostToday
}

func hasGVisorRuntimePod(rows []agentctl.RuntimePodRow) bool {
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.RuntimeClass), "gvisor") {
			return true
		}
	}
	return false
}

func (s *Server) handleWorkloads(w http.ResponseWriter, r *http.Request) {
	csrfToken, _ := r.Context().Value(csrfContextKey).(string)

	workloads, err := s.client.ListWorkloads(r.Context(), "")
	if err != nil {
		http.Error(w, "Failed to list workloads", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Workloads": workloads,
		"CSRFToken": csrfToken,
	}

	if err := s.tmpl.ExecuteTemplate(w, "workloads.html", data); err != nil {
		slog.Error("render workloads", "error", err)
	}
}

func (s *Server) handleDescribe(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	user := r.Context().Value(userContextKey).(*UserInfo)
	allowed, reason, err := s.authz.Authorize(r.Context(), user, "get", "agentworkloads", ns, name)
	if err != nil || !allowed {
		msg := "Access denied"
		if reason != "" {
			msg = reason
		}
		http.Error(w, msg, http.StatusForbidden)
		return
	}

	detail, err := s.client.DescribeWorkload(r.Context(), ns, name)
	if err != nil {
		slog.Error("describe workload", "error", err, "workload", name, "namespace", ns)
		http.Error(w, "Failed to describe workload", http.StatusInternalServerError)
		return
	}

	csrfToken, _ := r.Context().Value(csrfContextKey).(string)

	data := map[string]interface{}{
		"Detail":    detail,
		"CSRFToken": csrfToken,
	}

	if err := s.tmpl.ExecuteTemplate(w, "describe.html", data); err != nil {
		slog.Error("render describe", "error", err)
	}
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	user := r.Context().Value(userContextKey).(*UserInfo)
	allowed, reason, err := s.authz.Authorize(r.Context(), user, "update", "agentworkloads", ns, name)
	if err != nil || !allowed {
		msg := "Access denied"
		if reason != "" {
			msg = reason
		}
		http.Error(w, msg, http.StatusForbidden)
		return
	}

	result, err := s.client.ApproveWorkload(r.Context(), ns, name, user.Username)
	if err != nil {
		slog.Error("approve workload", "error", err, "workload", name, "namespace", ns, "user", user.Username)
		http.Error(w, "Failed to approve workload", http.StatusBadRequest)
		return
	}

	slog.Info("workload approved",
		"workload", result.Name,
		"namespace", result.Namespace,
		"user", user.Username,
		"argo_resumed", result.ArgoResumed,
	)

	// Return HTMX partial with updated workload list
	s.handleWorkloads(w, r)
}

func (s *Server) handleReject(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	user := r.Context().Value(userContextKey).(*UserInfo)
	allowed, reason, err := s.authz.Authorize(r.Context(), user, "update", "agentworkloads", ns, name)
	if err != nil || !allowed {
		msg := "Access denied"
		if reason != "" {
			msg = reason
		}
		http.Error(w, msg, http.StatusForbidden)
		return
	}

	rule := r.FormValue("rule")
	rejectReason := r.FormValue("reason")

	result, err := s.client.RejectWorkload(r.Context(), ns, name, rule, rejectReason, user.Username)
	if err != nil {
		slog.Error("reject workload", "error", err, "workload", name, "namespace", ns, "user", user.Username)
		http.Error(w, "Failed to reject workload", http.StatusBadRequest)
		return
	}

	slog.Info("workload rejected",
		"workload", result.Name,
		"namespace", result.Namespace,
		"user", user.Username,
		"rule", result.Rule,
		"reason", result.Reason,
	)

	s.handleWorkloads(w, r)
}

func (s *Server) handleCost(w http.ResponseWriter, r *http.Request) {
	costs, err := s.client.CostSummary(r.Context(), "", "", true)
	if err != nil {
		http.Error(w, "Failed to fetch cost data", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Costs": costs,
	}

	if err := s.tmpl.ExecuteTemplate(w, "cost.html", data); err != nil {
		slog.Error("render cost", "error", err)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.client.ClusterStatus(r.Context(), "")
	if err != nil {
		http.Error(w, "Failed to fetch status", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Status": status,
	}

	if err := s.tmpl.ExecuteTemplate(w, "status.html", data); err != nil {
		slog.Error("render status", "error", err)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		errorMsg := r.URL.Query().Get("error")
		data := map[string]interface{}{
			"Error": errorMsg,
		}
		if err := s.tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
			slog.Error("render login", "error", err)
		}
		return
	}

	// POST: set token cookie
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		http.Redirect(w, r, "/auth/login?error=empty_token", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "agentctl-token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
		MaxAge:   86400, // 24h
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "agentctl-token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}
