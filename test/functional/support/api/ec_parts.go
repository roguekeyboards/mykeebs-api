package api

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
)

// ECPartsClient calls the /v1/users/{userId}/ec-parts routes.
type ECPartsClient struct {
	client *Client
}

// NewECPartsClient returns a ready-to-use ECPartsClient.
func NewECPartsClient() *ECPartsClient {
	return &ECPartsClient{client: NewClient()}
}

// List calls GET /v1/users/{ownerID}/ec-parts with the given bearer token
// (empty for an anonymous request). A non-empty partType is sent as the
// part_type query parameter. The caller owns closing resp.Body.
func (c *ECPartsClient) List(ctx context.Context, ownerID, token, partType string) (*http.Response, error) {
	path := "/v1/users/" + ownerID + "/ec-parts"
	if partType != "" {
		path += "?" + url.Values{"part_type": []string{partType}}.Encode()
	}

	return c.client.Do(ctx, http.MethodGet, path, token, nil)
}

// Get calls GET /v1/users/{ownerID}/ec-parts/{id}. The caller owns closing
// resp.Body.
func (c *ECPartsClient) Get(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/users/"+ownerID+"/ec-parts/"+id, token, nil)
}

// Create calls POST /v1/users/{ownerID}/ec-parts with body as the raw JSON
// request body. The caller owns closing resp.Body.
func (c *ECPartsClient) Create(ctx context.Context, ownerID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/users/"+ownerID+"/ec-parts", token, bytes.NewBufferString(body))
}

// Update calls PUT /v1/users/{ownerID}/ec-parts/{id} with body as the raw
// JSON request body. The caller owns closing resp.Body.
func (c *ECPartsClient) Update(ctx context.Context, ownerID, id, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPut, "/v1/users/"+ownerID+"/ec-parts/"+id, token, bytes.NewBufferString(body))
}
