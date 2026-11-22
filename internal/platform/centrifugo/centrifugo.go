// Package centrifugo wraps the Centrifugo HTTP API client.
package centrifugo

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/centrifugal/gocent/v4"

	"github.com/furkanmeclis/messora-app-go/internal/config"
)

// Client is the server-side Centrifugo API client.
type Client struct {
	api *gocent.Client
}

// New builds a client for the configured HTTP API.
func New(c config.CentrifugoConfig) (*Client, error) {
	if c.APIURL == "" || !c.APIKey.IsSet() {
		return nil, errors.New("centrifugo: CENTRIFUGO_API_URL and CENTRIFUGO_API_KEY are required")
	}
	api, err := gocent.New(gocent.Config{
		APIEndpoint: c.APIURL,
		APIKey:      c.APIKey.Reveal(),
		HTTPClient:  &http.Client{Timeout: 5 * time.Second},
	})
	if err != nil {
		return nil, err
	}
	return &Client{api: api}, nil
}

// NodeInfo summarizes one Centrifugo node.
type NodeInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	NumClients uint32 `json:"num_clients"`
}

// Info returns cluster node information; it doubles as a connectivity and
// API-key check.
func (c *Client) Info(ctx context.Context) ([]NodeInfo, error) {
	res, err := c.api.Info(ctx, gocent.InfoRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]NodeInfo, 0, len(res.Nodes))
	for _, n := range res.Nodes {
		out = append(out, NodeInfo{Name: n.Name, Version: n.Version, NumClients: n.NumClients})
	}
	return out, nil
}
