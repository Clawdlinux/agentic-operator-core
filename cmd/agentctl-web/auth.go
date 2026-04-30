package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// UserInfo holds the authenticated user's identity.
type UserInfo struct {
	Username string
	UID      string
	Groups   []string
}

// TokenAuthenticator verifies bearer tokens via K8s TokenReview.
type TokenAuthenticator struct {
	client kubernetes.Interface
}

// NewTokenAuthenticator creates a TokenAuthenticator.
func NewTokenAuthenticator(client kubernetes.Interface) *TokenAuthenticator {
	return &TokenAuthenticator{client: client}
}

// Authenticate verifies a bearer token and returns user info.
func (a *TokenAuthenticator) Authenticate(ctx context.Context, token string) (*UserInfo, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("empty token")
	}

	review := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token: token,
		},
	}

	result, err := a.client.AuthenticationV1().TokenReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("token review failed: %w", err)
	}

	if !result.Status.Authenticated {
		return nil, fmt.Errorf("token not authenticated")
	}

	return &UserInfo{
		Username: result.Status.User.Username,
		UID:      result.Status.User.UID,
		Groups:   result.Status.User.Groups,
	}, nil
}

// Authorizer checks permissions via K8s SubjectAccessReview.
type Authorizer struct {
	client kubernetes.Interface
}

// NewAuthorizer creates an Authorizer.
func NewAuthorizer(client kubernetes.Interface) *Authorizer {
	return &Authorizer{client: client}
}

// Authorize checks whether a user can perform the given action.
func (a *Authorizer) Authorize(ctx context.Context, user *UserInfo, verb, resource, namespace, name string) (bool, string, error) {
	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   user.Username,
			Groups: user.Groups,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     "agentic.clawdlinux.org",
				Version:   "v1alpha1",
				Resource:  resource,
				Name:      name,
			},
		},
	}

	result, err := a.client.AuthorizationV1().SubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, "", fmt.Errorf("subject access review failed: %w", err)
	}

	if !result.Status.Allowed {
		reason := result.Status.Reason
		if reason == "" {
			reason = "access denied"
		}
		return false, reason, nil
	}

	return true, "", nil
}

// extractBearerToken gets the token from Authorization header or cookie.
func extractBearerToken(r *http.Request) string {
	// Try Authorization header first
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	// Fall back to cookie
	if cookie, err := r.Cookie("agentctl-token"); err == nil {
		return cookie.Value
	}
	return ""
}
