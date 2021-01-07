package models

import "fmt"

// Endpoint is representing host address
type Endpoint struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int32  `json:"port"`
}

// URI returns the full uri string protocol://host:port
func (endpoint *Endpoint) URI() string {
	if endpoint.Protocol != "" {
		return fmt.Sprintf("%s://%s:%d", endpoint.Protocol, endpoint.Host, endpoint.Port)
	}
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}

// Hostname returns the hostname part of endpoint (not including protocol)
func (endpoint *Endpoint) Hostname() string {
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}
