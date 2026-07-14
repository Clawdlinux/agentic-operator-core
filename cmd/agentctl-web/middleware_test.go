package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestAuthMiddlewareAllowsThemeAssets(t *testing.T) {
	called := false
	handler := AuthMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/theme/clawdlinux-theme.css", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("theme asset request did not reach the next handler")
	}
}

func TestAuthMiddlewareProtectsApplicationRoutes(t *testing.T) {
	called := false
	handler := AuthMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/agents", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	if location := response.Header().Get("Location"); location != "/auth/login" {
		t.Errorf("Location = %q, want %q", location, "/auth/login")
	}
	if called {
		t.Fatal("protected request reached the next handler")
	}
}

func TestAuthMiddlewareProtectsDemoRoute(t *testing.T) {
	kube := fake.NewClientset()
	kube.PrependReactor("create", "tokenreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authenticationv1.TokenReview{
			Status: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				User: authenticationv1.UserInfo{
					Username: "demo-reader",
				},
			},
		}, nil
	})
	authn := NewTokenAuthenticator(kube)

	t.Run("unauthenticated", func(t *testing.T) {
		called := false
		handler := AuthMiddleware(authn)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))

		request := httptest.NewRequest(http.MethodGet, "/demo", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
		}
		if location := response.Header().Get("Location"); location != "/auth/login" {
			t.Errorf("Location = %q, want %q", location, "/auth/login")
		}
		if called {
			t.Fatal("unauthenticated demo request reached the next handler")
		}
	})

	t.Run("authenticated", func(t *testing.T) {
		called := false
		handler := AuthMiddleware(authn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			user, ok := r.Context().Value(userContextKey).(*UserInfo)
			if !ok || user.Username != "demo-reader" {
				t.Fatalf("authenticated user = %#v, want demo-reader", user)
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		request := httptest.NewRequest(http.MethodGet, "/demo", nil)
		request.Header.Set("Authorization", "Bearer demo-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
		}
		if !called {
			t.Fatal("authenticated demo request did not reach the next handler")
		}
	})
}
