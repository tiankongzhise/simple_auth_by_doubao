package httpapi

import "net/http"

type apiUsage struct {
	ServiceName string          `json:"serviceName"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Conventions usageConvention `json:"conventions"`
	Endpoints   []usageEndpoint `json:"endpoints"`
	ErrorCodes  []usageError    `json:"errorCodes"`
	Notes       []string        `json:"notes"`
}

type usageConvention struct {
	ContentType       string   `json:"contentType"`
	HeaderNaming     string   `json:"headerNaming"`
	JSONNaming       string   `json:"jsonNaming"`
	TimeFormat       string   `json:"timeFormat"`
	ServiceNameRules []string `json:"serviceNameRules"`
}

type usageEndpoint struct {
	Method        string         `json:"method"`
	Path          string         `json:"path"`
	AuthRequired  bool           `json:"authRequired"`
	Description   string         `json:"description"`
	Headers       []usageField   `json:"headers,omitempty"`
	RequestBody   []usageField   `json:"requestBody,omitempty"`
	SuccessStatus int            `json:"successStatus"`
	ResponseBody  []usageField   `json:"responseBody"`
	Example       map[string]any `json:"example,omitempty"`
}

type usageField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type usageError struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, buildUsage())
}

func buildUsage() apiUsage {
	tokenResponse := []usageField{
		{Name: "accessToken", Type: "string", Required: true, Description: "当前生效的 access JWT"},
		{Name: "refreshToken", Type: "string", Required: true, Description: "当前生效的 refresh JWT"},
		{Name: "accessTokenExpiresAt", Type: "number", Required: true, Description: "access token 过期 Unix 秒级时间戳"},
		{Name: "accessTokenExpiresAtLocal", Type: "string", Required: true, Description: "access token 过期北京时间，格式 YYYY-MM-DD HH:mm:ss"},
		{Name: "refreshTokenExpiresAt", Type: "number", Required: true, Description: "refresh token 过期 Unix 秒级时间戳"},
		{Name: "refreshTokenExpiresAtLocal", Type: "string", Required: true, Description: "refresh token 过期北京时间，格式 YYYY-MM-DD HH:mm:ss"},
	}

	return apiUsage{
		ServiceName: "simple_auth_by_doubao",
		Version:     "1.0",
		Description: "简单鉴权服务公共使用说明，第三方服务可通过本接口发现可用 API、请求头、入参和返回值。",
		Conventions: usageConvention{
			ContentType:   "服务 API 使用 application/json；鉴权校验接口无请求体。",
			HeaderNaming: "请求头使用短横线命名，例如 Service-Name、Access-Token；不要使用下划线，避免 Nginx 默认丢弃。",
			JSONNaming:   "JSON 请求体和响应体使用 camelCase。",
			TimeFormat:   "时间字段同时返回 Unix 秒级时间戳和北京时间字符串。",
			ServiceNameRules: []string{
				"serviceName 支持中文。",
				"JSON 请求体中的 serviceName 可以直接使用 UTF-8。",
				"请求头 Service-Name 建议使用 URL 百分号编码，例如浏览器 encodeURIComponent(serviceName)。",
				"服务端会先 URL 解码再保存或比对 serviceName。",
			},
		},
		Endpoints: []usageEndpoint{
			{
				Method:       http.MethodGet,
				Path:         "/api/public/usage",
				AuthRequired: false,
				Description:  "获取本服务公共使用说明。",
				SuccessStatus: http.StatusOK,
				ResponseBody: []usageField{
					{Name: "serviceName", Type: "string", Required: true, Description: "本鉴权服务名称"},
					{Name: "version", Type: "string", Required: true, Description: "说明文档版本"},
					{Name: "conventions", Type: "object", Required: true, Description: "请求命名、时间和 serviceName 编码规则"},
					{Name: "endpoints", Type: "array", Required: true, Description: "可用接口说明"},
					{Name: "errorCodes", Type: "array", Required: true, Description: "通用错误码"},
				},
			},
			{
				Method:       http.MethodPost,
				Path:         "/api/token/exchange",
				AuthRequired: false,
				Description:  "使用永久授权码换取 access token 和 refresh token；成功后会生成当前生效 token 组。",
				Headers: []usageField{
					{Name: "Content-Type", Type: "string", Required: true, Description: "固定为 application/json"},
					{Name: "Origin", Type: "string", Required: false, Description: "服务来源 origin，Origin 或 Referer 至少提供一个"},
					{Name: "Referer", Type: "string", Required: false, Description: "没有 Origin 时使用 Referer 提取 origin"},
					{Name: "model", Type: "string", Required: false, Description: "仅开发模式可传 dev；满足配置和 IP 白名单时跳过服务地址校验"},
				},
				RequestBody: []usageField{
					{Name: "serviceName", Type: "string", Required: true, Description: "服务名称，支持中文和 URL 百分号编码"},
					{Name: "authorizationCode", Type: "string", Required: true, Description: "32 位永久授权码"},
				},
				SuccessStatus: http.StatusOK,
				ResponseBody:  tokenResponse,
				Example: map[string]any{
					"requestBody": map[string]any{
						"serviceName":       "订单服务",
						"authorizationCode": "abcd1234abcd1234abcd1234abcd1234",
					},
				},
			},
			{
				Method:       http.MethodPost,
				Path:         "/api/token/refresh",
				AuthRequired: false,
				Description:  "使用当前 refresh token 成对刷新 access token 和 refresh token；旧 token 立即失效。",
				Headers: []usageField{
					{Name: "Content-Type", Type: "string", Required: true, Description: "固定为 application/json"},
					{Name: "Origin", Type: "string", Required: false, Description: "服务来源 origin，Origin 或 Referer 至少提供一个"},
					{Name: "Referer", Type: "string", Required: false, Description: "没有 Origin 时使用 Referer 提取 origin"},
					{Name: "model", Type: "string", Required: false, Description: "仅开发模式可传 dev；满足配置和 IP 白名单时跳过服务地址校验"},
				},
				RequestBody: []usageField{
					{Name: "serviceName", Type: "string", Required: true, Description: "服务名称，支持中文和 URL 百分号编码"},
					{Name: "refreshToken", Type: "string", Required: true, Description: "当前生效的 refresh JWT"},
				},
				SuccessStatus: http.StatusOK,
				ResponseBody:  tokenResponse,
				Example: map[string]any{
					"requestBody": map[string]any{
						"serviceName":  "订单服务",
						"refreshToken": "<currentRefreshJwt>",
					},
				},
			},
			{
				Method:       http.MethodPost,
				Path:         "/api/auth/verify",
				AuthRequired: false,
				Description:  "校验当前 access token 是否属于注册服务且仍然生效；超过服务 QPS/QPM 限额会拒绝。",
				Headers: []usageField{
					{Name: "Service-Name", Type: "string", Required: true, Description: "服务名称；中文建议 URL 百分号编码"},
					{Name: "Access-Token", Type: "string", Required: true, Description: "当前生效的 access JWT"},
					{Name: "Origin", Type: "string", Required: false, Description: "服务来源 origin，Origin 或 Referer 至少提供一个"},
					{Name: "Referer", Type: "string", Required: false, Description: "没有 Origin 时使用 Referer 提取 origin"},
					{Name: "model", Type: "string", Required: false, Description: "仅开发模式可传 dev；满足配置和 IP 白名单时跳过服务地址校验"},
				},
				SuccessStatus: http.StatusOK,
				ResponseBody: []usageField{
					{Name: "ok", Type: "boolean", Required: true, Description: "是否校验通过"},
					{Name: "serviceName", Type: "string", Required: true, Description: "解码后的服务名称"},
				},
				Example: map[string]any{
					"headers": map[string]any{
						"Service-Name": "%E8%AE%A2%E5%8D%95%E6%9C%8D%E5%8A%A1",
						"Access-Token": "<currentAccessJwt>",
						"Origin":       "https://order.example.com",
					},
				},
			},
		},
		ErrorCodes: []usageError{
			{Status: http.StatusBadRequest, Description: "请求体格式错误或字段校验失败"},
			{Status: http.StatusUnauthorized, Description: "管理员未登录、密码错误、token 错误或过期"},
			{Status: http.StatusForbidden, Description: "来源地址不匹配或开发模式 IP 不允许"},
			{Status: http.StatusNotFound, Description: "服务不存在"},
			{Status: http.StatusConflict, Description: "服务名称或服务地址重复"},
			{Status: http.StatusTooManyRequests, Description: "超过 QPS 或 QPM 限额"},
			{Status: http.StatusInternalServerError, Description: "服务器内部错误"},
		},
		Notes: []string{
			"正式模式下，服务 API 优先校验 Origin，其次校验 Referer 的 origin，必须与注册服务地址一致。",
			"model: dev 只跳过服务地址校验，不跳过服务名、token 和限流校验，禁止生产使用。",
			"token 为 JWT；当前生效 token 以 PostgreSQL 为准，Redis 仅作缓存。",
		},
	}
}
