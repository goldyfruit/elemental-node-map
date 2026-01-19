package k8s

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type ErrorKind string

const (
	ErrKubeconfigNotFound ErrorKind = "kubeconfig_not_found"
	ErrKubeconfigInvalid  ErrorKind = "kubeconfig_invalid"
	ErrContextNotFound    ErrorKind = "context_not_found"
	ErrAuthFailed         ErrorKind = "auth_failed"
	ErrForbidden          ErrorKind = "forbidden"
	ErrClusterUnreachable ErrorKind = "cluster_unreachable"
	ErrUnknown            ErrorKind = "unknown"
)

type ConfigError struct {
	Kind  ErrorKind
	Paths []string
	Err   error
}

func (e *ConfigError) Error() string {
	suffix := ""
	if len(e.Paths) > 0 {
		suffix = fmt.Sprintf(" (%s)", strings.Join(e.Paths, ", "))
	}
	switch e.Kind {
	case ErrKubeconfigNotFound:
		return fmt.Sprintf("kubeconfig not found%s", suffix)
	case ErrKubeconfigInvalid:
		return fmt.Sprintf("invalid kubeconfig%s: %v", suffix, e.Err)
	case ErrContextNotFound:
		if e.Err != nil {
			return fmt.Sprintf("kubeconfig context not found%s: %v", suffix, e.Err)
		}
		return fmt.Sprintf("kubeconfig context not found%s", suffix)
	default:
		return fmt.Sprintf("kubeconfig error%s: %v", suffix, e.Err)
	}
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

type APIError struct {
	Kind ErrorKind
	Err  error
}

func (e *APIError) Error() string {
	switch e.Kind {
	case ErrAuthFailed:
		return "kubernetes authentication failed"
	case ErrForbidden:
		return "kubernetes authorization failed"
	case ErrClusterUnreachable:
		return "kubernetes cluster unreachable"
	default:
		return fmt.Sprintf("kubernetes API error: %v", e.Err)
	}
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func classifyK8sError(err error) *APIError {
	if err == nil {
		return nil
	}
	if k8serrors.IsUnauthorized(err) {
		return &APIError{Kind: ErrAuthFailed, Err: err}
	}
	if k8serrors.IsForbidden(err) {
		return &APIError{Kind: ErrForbidden, Err: err}
	}
	if isUnreachable(err) {
		return &APIError{Kind: ErrClusterUnreachable, Err: err}
	}
	return &APIError{Kind: ErrUnknown, Err: err}
}

func isUnreachable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if strings.Contains(strings.ToLower(err.Error()), "connection refused") {
		return true
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such host") {
		return true
	}
	if strings.Contains(strings.ToLower(err.Error()), "i/o timeout") {
		return true
	}
	if strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
		return true
	}
	return false
}
