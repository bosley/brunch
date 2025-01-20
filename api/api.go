package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Opts struct {
	Https      bool
	SkipVerify bool
}

func NewWithPassword(address, username, password string, opts Opts) (*ApiClient, error) {
	client := &http.Client{}

	if opts.Https {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.SkipVerify,
			},
		}
		client = &http.Client{Transport: tr}
		if !strings.HasPrefix(address, "https://") {
			address = "https://" + address
		}
	} else if !strings.HasPrefix(address, "http://") {
		address = "http://" + address
	}

	authReq := BrunchAuthRequest{
		Username: username,
		Password: password,
	}

	jsonData, err := json.Marshal(authReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/auth", address), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	var authResp BrunchAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.Code != http.StatusOK {
		return nil, fmt.Errorf("authentication failed: %s", authResp.Message)
	}

	return &ApiClient{
		token:      authResp.Token,
		skipVerify: opts.SkipVerify,
		https:      opts.Https,
		address:    address,
	}, nil
}

func (c *ApiClient) Query(op BrunchOp, key, value string) (BrunchQueryResponse, error) {
	client := &http.Client{}

	if c.https {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: c.skipVerify,
			},
		}
		client = &http.Client{Transport: tr}
	}

	queryReq := BrunchQueryRequest{
		Token: c.token,
		Op:    op,
		Key:   key,
		Value: value,
	}

	jsonData, err := json.Marshal(queryReq)
	if err != nil {
		return BrunchQueryResponse{}, fmt.Errorf("failed to marshal query request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/brunch", c.address), bytes.NewBuffer(jsonData))
	if err != nil {
		return BrunchQueryResponse{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return BrunchQueryResponse{}, fmt.Errorf("failed to send query request: %w", err)
	}
	defer resp.Body.Close()

	var queryResp BrunchQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return BrunchQueryResponse{}, fmt.Errorf("failed to decode query response: %w", err)
	}

	if queryResp.Code != http.StatusOK && queryResp.Code != http.StatusCreated {
		return queryResp, fmt.Errorf("query failed: %s", queryResp.Message)
	}

	return queryResp, nil
}
