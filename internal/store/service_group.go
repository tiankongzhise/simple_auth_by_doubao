package store

type ServiceGroup struct {
	ID                        int64     `json:"id"`
	ServiceGroupName          string    `json:"serviceGroupName"`
	ServiceGroupURL           string    `json:"serviceGroupUrl"`
	AuthorizationCodeHash     string    `json:"-"`
	AuthorizationCodeMasked   string    `json:"authorizationCodeMasked"`
	AccessToken               string    `json:"accessToken"`
	AccessTokenExpiresAt      int64     `json:"accessTokenExpiresAt"`
	AccessTokenExpiresAtLocal string    `json:"accessTokenExpiresAtLocal"`
	TokenVersion              int64     `json:"tokenVersion"`
	Services                  []Service `json:"services"`
	ServiceIDs                []int64   `json:"serviceIds"`
	CreatedAt                 int64     `json:"createdAt"`
	CreatedAtLocal            string    `json:"createdAtLocal"`
	UpdatedAt                 int64     `json:"updatedAt"`
	UpdatedAtLocal            string    `json:"updatedAtLocal"`
}

type CreateServiceGroupInput struct {
	ServiceGroupName        string
	AuthorizationCodeHash   string
	AuthorizationCodeMasked string
	ServiceIDs              []int64
}

type UpdateServiceGroupInput struct {
	ServiceGroupName string
	ServiceIDs       []int64
}
