package store

type Service struct {
	ID                         int64  `json:"id"`
	ServiceName                string `json:"serviceName"`
	ServiceURL                 string `json:"serviceUrl"`
	AuthorizationCodeHash      string `json:"-"`
	AuthorizationCodeMasked    string `json:"authorizationCodeMasked"`
	QPS                        int    `json:"qps"`
	QPM                        int    `json:"qpm"`
	AccessToken                string `json:"accessToken"`
	RefreshToken               string `json:"refreshToken"`
	AccessTokenExpiresAt       int64  `json:"accessTokenExpiresAt"`
	AccessTokenExpiresAtLocal  string `json:"accessTokenExpiresAtLocal"`
	RefreshTokenExpiresAt      int64  `json:"refreshTokenExpiresAt"`
	RefreshTokenExpiresAtLocal string `json:"refreshTokenExpiresAtLocal"`
	TokenVersion               int64  `json:"tokenVersion"`
	CreatedAt                  int64  `json:"createdAt"`
	CreatedAtLocal             string `json:"createdAtLocal"`
	UpdatedAt                  int64  `json:"updatedAt"`
	UpdatedAtLocal             string `json:"updatedAtLocal"`
}

type CreateServiceInput struct {
	ServiceName             string
	ServiceURL              string
	AuthorizationCodeHash   string
	AuthorizationCodeMasked string
	QPS                     int
	QPM                     int
}

type UpdateServiceInput struct {
	ServiceName string
	ServiceURL  string
	QPS         int
	QPM         int
}
