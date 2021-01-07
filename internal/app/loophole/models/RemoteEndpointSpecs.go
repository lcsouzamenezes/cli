package models

// RemoteEndpointSpecs is collection of parameters used to describe
// configuration for public endpoint
type RemoteEndpointSpecs struct {
	GatewayEndpoint   Endpoint `json:"gatewayEndpoint"`
	APIEndpoint       Endpoint `json:"apiEndpoint"`
	IdentityFile      string   `json:"identityFile"`
	SiteID            string   `json:"siteId"`
	BasicAuthUsername string   `json:"basicAuthUsername"`
	BasicAuthPassword string   `json:"basicAuthPassword"`
}
