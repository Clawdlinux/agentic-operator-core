package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/shreyansh/agentic-operator/pkg/agentctl"
)

// Server holds the web server dependencies.
type Server struct {
	client *agentctl.Client
	authz  *Authorizer
	tmpl   *template.Template
}

// NewServer creates a Server with parsed templates.
func NewServer(client *agentctl.Client, authz *Authorizer, tmplFS fs.FS) (*Server, error) {
	funcMap := template.FuncMap{
		"phaseIcon": phaseIcon,
		"lower":     strings.ToLower,
		"safeText":  agentctl.SafeText,
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

	return &Server{client: client, authz: authz, tmpl: tmpl}, nil
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

func phaseIcon(phase string) string {
	switch phase {
	case "Completed":
		return "✓"
	case "Running":
		return "▶"
	case "Failed":
		return "✗"
	case "PendingApproval":
		return "⏸"
	case "Suspended":
		return "⏸"
	default:
		return "●"
	}
}
