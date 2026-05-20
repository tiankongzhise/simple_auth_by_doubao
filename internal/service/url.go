package service

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func NormalizeOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: serviceUrl is required", ErrBadRequest)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: invalid serviceUrl", ErrBadRequest)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: serviceUrl only supports http or https", ErrBadRequest)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", fmt.Errorf("%w: serviceUrl host is required", ErrBadRequest)
	}
	if port := parsed.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	return parsed.Scheme + "://" + host, nil
}

func OriginFromRequestHeaders(origin string, referer string) (string, error) {
	if strings.TrimSpace(origin) != "" {
		return NormalizeOrigin(origin)
	}
	if strings.TrimSpace(referer) == "" {
		return "", fmt.Errorf("%w: Origin or Referer is required", ErrForbidden)
	}
	return NormalizeOrigin(referer)
}
