package odoo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/oauth2/clientcredentials"
)

type Options struct {
	BaseURL string
	DB      string

	ClientID     string
	ClientSecret string
	TokenURL     string
}

type Client struct {
	http    *http.Client
	baseURL *url.URL
	db      string
}

type InstanceEvent struct {
	ProductID            string `json:"product_id"`
	InstanceID           string `json:"instance_id"`
	SalesOrderID         string `json:"sales_order_id"`
	ItemDescription      string `json:"item_description,omitempty"`
	ItemGroupDescription string `json:"item_group_description,omitempty"`
	UnitID               string `json:"unit_id"`
	EventType            string `json:"event_type"`
	Size                 string `json:"size,omitempty"`
	Timestamp            string `json:"timestamp"`
}

func NewClient(ctx context.Context, opts Options) (*Client, error) {
	if opts.BaseURL == "" || opts.DB == "" {
		return nil, errors.New("BaseURL and DB are required")
	}
	if opts.ClientID == "" || opts.ClientSecret == "" || opts.TokenURL == "" {
		return nil, errors.New("ClientID, ClientSecret, and TokenURL are required")
	}

	u, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid BaseURL: %w", err)
	}

	cc := clientcredentials.Config{
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		TokenURL:     opts.TokenURL,
	}

	hc := cc.Client(ctx)

	return &Client{
		http:    hc,
		baseURL: u,
		db:      opts.DB,
	}, nil
}

func (c *Client) SendInstanceEvent(ctx context.Context, event InstanceEvent) error {
	endpoint, err := url.JoinPath(c.baseURL.String(), "/api/v2/billing/product_usage_event_POST")
	if err != nil {
		return fmt.Errorf("join path: %w", err)
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}

	q := u.Query()
	q.Set("db", c.db)
	u.RawQuery = q.Encode()

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error while sending odoo instance event: status %d", resp.StatusCode)
	}

	return nil
}
