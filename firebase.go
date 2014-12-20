// Package firebase impleements a RESTful client for Firebase.
package firebase

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/facebookgo/httpcontrol"
)

// Api is the interface for interacting with Firebase.
// Consumers of this package can mock this interface for testing purposes.
type Api interface {
	Call(method, path, auth string, body interface{}, params map[string]string, dest interface{}) error
}

// Client is the Firebase client.
type Client struct {
	// Url is the client's base URL used for all calls.
	Url string

	// Auth is authentication token used when making calls.
	// The token is optional and can also be overwritten on an individual
	// call basis via params.
	Auth string

	// An error occurred at some point in the call chain
	LastError error

	// api is the underlying client used to make calls.
	api Api
}

// Rules is the structure for security rules.
type Rules map[string]interface{}

// f is the internal implementation of the Firebase API client.
type f struct{}

// suffix is the Firebase suffix for invoking their API via HTTP
const SUFFIX = ".json"

var (
	connectTimeout   = time.Duration(10 * time.Second) // timeout for http connection
	readWriteTimeout = time.Duration(10 * time.Second) // timeout for http read/write
)

// httpClient is the HTTP client used to make calls to Firebase
var httpClient = newTimeoutClient(connectTimeout, readWriteTimeout)

// Init initializes the Firebase client with a given root url and optional auth token.
// Also takes an error channel to pass errors along when chaining together calls.
// The initialization can also pass a mock api for testing purposes.
func NewClient(root, auth string, api Api) *Client {
	if api == nil {
		api = new(f)
	}

	return &Client{Url: root, Auth: auth, api: api}
}

// Value returns the value of of the current Url.
func (c *Client) Value(destination interface{}) error {
	// if we have not yet performed a look-up, do it so a value is returned
	err := c.api.Call("GET", c.Url, c.Auth, nil, nil, destination)
	if err != nil {
		return err
	}
	return nil
}

// Child returns a populated pointer for a given path.
func (c *Client) Child(path string) *Client {
	u := c.Url + "/" + path
	return &Client{
		api:  c.api,
		Auth: c.Auth,
		Url:  u,
	}
}

// Push creates a new value under the current root url.
// A populated pointer with that value is also returned.
func (c *Client) Push(value interface{}, params map[string]string) (*Client, error) {
	res := map[string]string{}
	err := c.api.Call("POST", c.Url, c.Auth, value, params, &res)
	if err != nil {
		return nil, err
	}

	return &Client{
		api:  c.api,
		Auth: c.Auth,
		Url:  c.Url + "/" + res["name"],
	}, nil
}

// Set overwrites the value at the specified path and returns populated pointer
// for the updated path.
func (c *Client) Set(path string, value interface{}, params map[string]string) (*Client, error) {
	u := c.Url + "/" + path

	err := c.api.Call("PUT", u, c.Auth, value, params, nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		api:  c.api,
		Auth: c.Auth,
		Url:  u,
	}, nil
}

// Update performs a partial update with the given value at the specified path.
func (c *Client) Update(path string, value interface{}, params map[string]string) error {
	err := c.api.Call("PATCH", c.Url+"/"+path, c.Auth, value, params, nil)
	return err
}

// Remove deletes the data at the given path.
func (c *Client) Remove(path string, params map[string]string) error {
	err := c.api.Call("DELETE", c.Url+"/"+path, c.Auth, nil, params, nil)

	return err
}

// Rules returns the security rules for the database.
func (c *Client) Rules(params map[string]string) (*Rules, error) {
	res := &Rules{}
	err := c.api.Call("GET", c.Url+"/.settings/rules", c.Auth, nil, params, res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SetRules overwrites the existing security rules with the new rules given.
func (c *Client) SetRules(rules *Rules, params map[string]string) error {
	err := c.api.Call("PUT", c.Url+"/.settings/rules", c.Auth, rules, params, nil)

	return err
}

// Call invokes the appropriate HTTP method on a given Firebase URL.
func (f *f) Call(method, path, auth string, body interface{}, params map[string]string, dest interface{}) error {
	/*
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}*/

	path += SUFFIX
	qs := url.Values{}

	// if the client has an auth, set it as a query string.
	// the caller can also override this on a per-call basis
	// which will happen via params below
	if len(auth) > 0 {
		qs.Set("auth", auth)
	}

	for k, v := range params {
		qs.Set(k, v)
	}

	if len(qs) > 0 {
		path += "?" + qs.Encode()
	}

	encodedBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(encodedBody))
	if err != nil {
		return err
	}

	req.Close = true

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		err = errors.New(res.Status)
		return err
	}

	if dest != nil && res.ContentLength != 0 {
		decoder := json.NewDecoder(res.Body)
		err = decoder.Decode(dest)
		if err != nil {
			return err
		}
	}

	return nil
}

func newTimeoutClient(connectTimeout time.Duration, readWriteTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &httpcontrol.Transport{
			RequestTimeout: readWriteTimeout,
			DialTimeout:    connectTimeout,
			MaxTries:       3,
		},
	}
}
