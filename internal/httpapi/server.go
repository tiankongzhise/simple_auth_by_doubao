package httpapi

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"simple_auth_by_doubao/internal/auth"
	"simple_auth_by_doubao/internal/config"
	"simple_auth_by_doubao/internal/service"
	"simple_auth_by_doubao/internal/token"
)

//go:embed web/*
var webFiles embed.FS

type Server struct {
	cfg      *config.Config
	services *service.Manager
	tokens   *token.Manager
	mux      *http.ServeMux
}

func NewServer(cfg *config.Config, services *service.Manager, tokens *token.Manager) http.Handler {
	s := &Server{
		cfg:      cfg,
		services: services,
		tokens:   tokens,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	s.mux.HandleFunc("GET /api/public/usage", s.handleUsage)
	s.mux.HandleFunc("POST /api/admin/login", s.handleAdminLogin)
	s.mux.HandleFunc("POST /api/admin/logout", s.requireAdmin(s.handleAdminLogout))
	s.mux.HandleFunc("GET /api/admin/services", s.requireAdmin(s.handleAdminListServices))
	s.mux.HandleFunc("POST /api/admin/services", s.requireAdmin(s.handleAdminCreateService))
	s.mux.HandleFunc("PUT /api/admin/services/{id}", s.requireAdmin(s.handleAdminUpdateService))
	s.mux.HandleFunc("DELETE /api/admin/services/{id}", s.requireAdmin(s.handleAdminDeleteService))
	s.mux.HandleFunc("POST /api/admin/services/{id}/tokens/refresh", s.requireAdmin(s.handleAdminRefreshTokens))
	s.mux.HandleFunc("GET /api/admin/service-groups", s.requireAdmin(s.handleAdminListServiceGroups))
	s.mux.HandleFunc("POST /api/admin/service-groups", s.requireAdmin(s.handleAdminCreateServiceGroup))
	s.mux.HandleFunc("PUT /api/admin/service-groups/{id}", s.requireAdmin(s.handleAdminUpdateServiceGroup))
	s.mux.HandleFunc("POST /api/admin/service-groups/{id}/tokens/refresh", s.requireAdmin(s.handleAdminRefreshServiceGroupToken))
	s.mux.HandleFunc("POST /api/token/exchange", s.handleExchangeToken)
	s.mux.HandleFunc("POST /api/token/refresh", s.handleRefreshToken)
	s.mux.HandleFunc("POST /api/service-groups/token/latest", s.handleLatestServiceGroupToken)
	s.mux.HandleFunc("POST /api/auth/verify", s.handleVerify)
	s.mountUI()
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username != s.cfg.AdminUser || !auth.CheckPassword(s.cfg.AdminPasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}
	session, expiresAt, err := s.tokens.IssueAdmin(req.Username, s.cfg.AdminSessionTTL, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "创建管理员会话失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "authSession",
		Value:    session,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "登录成功"})
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "authSession",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "已退出登录"})
}

func (s *Server) handleAdminListServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.services.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询服务列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": services})
}

func (s *Server) handleAdminCreateService(w http.ResponseWriter, r *http.Request) {
	var req service.CreateServiceInput
	if !decodeJSON(w, r, &req) {
		return
	}
	svc, code, err := s.services.CreateService(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"service":           svc,
		"authorizationCode": code,
	})
}

func (s *Server) handleAdminUpdateService(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	var req service.UpdateServiceInput
	if !decodeJSON(w, r, &req) {
		return
	}
	svc, err := s.services.UpdateService(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": svc})
}

func (s *Server) handleAdminDeleteService(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	if err := s.services.DeleteService(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminRefreshTokens(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	tokens, err := s.services.RefreshTokensForService(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleAdminListServiceGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.services.ListServiceGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list service groups failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"serviceGroups": groups})
}

func (s *Server) handleAdminCreateServiceGroup(w http.ResponseWriter, r *http.Request) {
	var req service.CreateServiceGroupInput
	if !decodeJSON(w, r, &req) {
		return
	}
	group, code, err := s.services.CreateServiceGroup(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"serviceGroup":      group,
		"authorizationCode": code,
	})
}

func (s *Server) handleAdminUpdateServiceGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	var req service.UpdateServiceGroupInput
	if !decodeJSON(w, r, &req) {
		return
	}
	group, err := s.services.UpdateServiceGroup(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"serviceGroup": group})
}

func (s *Server) handleAdminRefreshServiceGroupToken(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	tokens, err := s.services.RefreshTokenForServiceGroup(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleExchangeToken(w http.ResponseWriter, r *http.Request) {
	var req service.ExchangeTokenInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Origin = r.Header.Get("Origin")
	req.Referer = r.Header.Get("Referer")
	req.RemoteAddr = r.RemoteAddr
	req.Model = r.Header.Get("model")

	tokens, err := s.services.ExchangeToken(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req service.RefreshTokenInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Origin = r.Header.Get("Origin")
	req.Referer = r.Header.Get("Referer")
	req.RemoteAddr = r.RemoteAddr
	req.Model = r.Header.Get("model")

	tokens, err := s.services.RefreshToken(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleLatestServiceGroupToken(w http.ResponseWriter, r *http.Request) {
	var req service.LatestServiceGroupTokenInput
	if !decodeJSON(w, r, &req) {
		return
	}
	tokens, err := s.services.LatestServiceGroupToken(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	req := service.VerifyInput{
		ServiceName:       r.Header.Get("Service-Name"),
		TargetServiceName: r.Header.Get("Target-Service-Name"),
		AccessToken:       r.Header.Get("Access-Token"),
		Origin:            r.Header.Get("Origin"),
		Referer:           r.Header.Get("Referer"),
		RemoteAddr:        r.RemoteAddr,
		Model:             r.Header.Get("model"),
	}
	result, err := s.services.Verify(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) mountUI() {
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	static := http.FileServer(http.FS(sub))
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", static))
	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, sub, "index.html")
	})
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("authSession")
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		if _, err := s.tokens.ParseAdmin(cookie.Value); err != nil {
			writeError(w, http.StatusUnauthorized, "管理员会话无效或已过期")
			return
		}
		next(w, r)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		writeError(w, http.StatusBadRequest, "请求 JSON 格式错误")
		return false
	}
	return true
}

func parsePathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "服务 ID 无效")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBadRequest):
		writeError(w, http.StatusBadRequest, cleanError(err))
	case errors.Is(err, service.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, cleanError(err))
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, cleanError(err))
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "服务不存在")
	case errors.Is(err, service.ErrConflict):
		writeError(w, http.StatusConflict, "服务名称或服务地址已存在")
	case errors.Is(err, service.ErrTooManyRequests):
		writeError(w, http.StatusTooManyRequests, cleanError(err))
	default:
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
	}
}

func cleanError(err error) string {
	message := err.Error()
	for _, prefix := range []string{
		service.ErrBadRequest.Error() + ": ",
		service.ErrUnauthorized.Error() + ": ",
		service.ErrForbidden.Error() + ": ",
		service.ErrTooManyRequests.Error() + ": ",
	} {
		message = strings.TrimPrefix(message, prefix)
	}
	if message == "" {
		return fmt.Sprintf("%v", err)
	}
	return message
}
