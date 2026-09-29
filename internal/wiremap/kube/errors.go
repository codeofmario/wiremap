package kube

import (
	"fmt"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// WrapError maps a Kubernetes API error to a typed app error, keeping RBAC
// denials and missing objects distinguishable from server failures.
func WrapError(err error, action string) error {
	msg := fmt.Sprintf("failed to %s: %s", action, err)
	switch {
	case k8serrors.IsNotFound(err):
		return apperrors.NotFound(msg)
	case k8serrors.IsForbidden(err):
		return apperrors.Forbidden(msg)
	default:
		return apperrors.Internal(msg)
	}
}

// IsOptional reports errors that should hide an optional part of a view rather
// than fail it: the user may lack RBAC for it, or the API may not exist.
func IsOptional(err error) bool {
	return k8serrors.IsForbidden(err) || k8serrors.IsNotFound(err)
}
