// Package traefikapi reads live routing state from the Traefik API.
package traefikapi

import (
	"encoding/json"
	"net/http"
	"time"
)

// Router is one Traefik HTTP router.
type Router struct {
	Name    string `json:"name"`
	Rule    string `json:"rule"`
	Service string `json:"service"`
	Status  string `json:"status"`
}

// Client talks to the Traefik API (default http://127.0.0.1:8080).
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// Default returns a Client pointed at the local berth proxy API.
func Default() *Client {
	return &Client{BaseURL: "http://127.0.0.1:8080", HTTP: &http.Client{Timeout: 2 * time.Second}}
}

// Routers returns all HTTP routers Traefik currently knows about.
func (c *Client) Routers() ([]Router, error) {
	resp, err := c.HTTP.Get(c.BaseURL + "/api/http/routers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rs []Router
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return nil, err
	}
	return rs, nil
}
