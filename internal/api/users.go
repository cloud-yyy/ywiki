package api

import (
	"context"
	"net/http"
)

// GetCurrentUser returns the user the token authenticates as.
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var user User
	if err := c.do(ctx, request{method: http.MethodGet, path: "/users/me"}, &user); err != nil {
		return nil, err
	}

	return &user, nil
}
