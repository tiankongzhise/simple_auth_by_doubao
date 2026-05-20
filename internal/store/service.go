package store

import "time"

type Service struct {
	ID                      int64      `json:"id"`
	ServiceName             string     `json:"serviceName"`
	ServiceURL              string     `json:"serviceUrl"`
	AuthorizationCodeHash   string     `json:"-"`
	AuthorizationCodeMasked string     `json:"authorizationCodeMasked"`
	QPS                     int        `json:"qps"`
	QPM                     int        `json:"qpm"`
	AccessToken             string     `json:"accessToken"`
	RefreshToken            string     `json:"refreshToken"`
	AccessTokenExpiresAt    *time.Time `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt   *time.Time `json:"refreshTokenExpiresAt"`
	TokenVersion            int64      `json:"tokenVersion"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
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
