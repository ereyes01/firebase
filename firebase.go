// Package firebase gives a thin wrapper around the firebase REST API. It tries
// to mirror the Official firebase API somewhat closely. https://www.firebase.com/docs/web/api/
package firebase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/facebookgo/httpcontrol"
)

// Timestamp is a time.Time with support for going from and to firebase
// ServerValue.TIMESTAMP fields.
//
// Thanks to Gal Ben-Haim for the inspiration
// https://medium.com/coding-and-deploying-in-the-cloud/time-stamps-in-golang-abcaf581b72f
type Timestamp time.Time

const milliDivider = 1000000

func (t *Timestamp) MarshalJSON() ([]byte, error) {
	ts := time.Time(*t).UnixNano() / milliDivider // Milliseconds
	stamp := fmt.Sprint(ts)

	return []byte(stamp), nil
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	ts, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return err
	}

	seconds := int64(ts) / 1000
	nanoseconds := (int64(ts) % 1000) * milliDivider
	*t = Timestamp(time.Unix(seconds, nanoseconds))

	return nil
}

func (t Timestamp) String() string {
	return time.Time(t).String()
}

type serverValue struct {
	Value string `json:".sv"`
}

// Use this value to represent a Firebase server timestamp in a data structure.
// This should be used when you're sending data to Firebase, as opposed to
// the Timestamp type.
var ServerTimestamp serverValue = serverValue{"timestamp"}

// KeyedValue is a type that is used to retain the key when pushing
// values onto a channel. It is used by Client#Iterator().
type KeyedValue struct {
	Key   string
	Value interface{}
}

// Api is the internal interface for interacting with Firebase.
// Consumers of this package can mock this interface for testing purposes, regular
// consumers can just use the default implementation and can ignore this completely.
// Arguments are as follows:
//  - `method`: The http method for this call
//  - `path`: The full firebase url to call
//  - `body`: Data to be marshalled to JSON (it's the responsibility of Call to do the marshalling and unmarshalling)
//  - `params`: Additional parameters to be passed to firebase
//  - `dest`: The object to save the unmarshalled response body to.
//    It's up to this method to unmarshal correctly, the default implemenation just uses `json.Unmarshal`
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

	// api is the underlying client used to make calls.
	api Api
}

// Rules is the structure for security rules.
type Rules map[string]interface{}

// f is the internal implementation of the Firebase API client.
type f struct{}

var (
	connectTimeout   = time.Duration(10 * time.Second) // timeout for http connection
	readWriteTimeout = time.Duration(10 * time.Second) // timeout for http read/write
)

// httpClient is the HTTP client used to make calls to Firebase with the default API
var httpClient = newTimeoutClient(connectTimeout, readWriteTimeout)

// Initializes the Firebase client with a given root url and optional auth token.
// The initialization can also pass a mock api for testing purposes, most consumers
// will just pass `nil` for the `api` parameter.
func NewClient(root, auth string, api Api) *Client {
	if api == nil {
		api = new(f)
	}

	return &Client{Url: root, Auth: auth, api: api}
}

// Gets the value referenced by the client and unmarshals it into
// the passed in destination.
func (c *Client) Value(destination interface{}) error {
	err := c.api.Call("GET", c.Url, c.Auth, nil, nil, destination)
	if err != nil {
		return err
	}
	return nil
}

// Iterator returns a channel that will emit objects in key order.
// TODO: Support more ordering options
func (c *Client) Iterator() <-chan *KeyedValue {
	out := make(chan *KeyedValue)
	go func() {
		unorderedVal := map[string]interface{}{}
		c.Value(unorderedVal)
		keys := make([]string, len(unorderedVal))
		i := 0
		for key, _ := range unorderedVal {
			keys[i] = key
			i++
		}
		sort.Strings(keys)
		for _, key := range keys {
			out <- &KeyedValue{
				Key:   key,
				Value: unorderedVal[key],
			}
		}
		close(out)
	}()
	return out
}

// Child returns a reference to the child specified by `path`. This does not
// actually make a request to firebase, but you can then manipulate the reference
// by calling one of the other methods (such as `Value`, `Update`, or `Set`).
func (c *Client) Child(path string) *Client {
	u := c.Url + "/" + path
	return &Client{
		api:  c.api,
		Auth: c.Auth,
		Url:  u,
	}
}

// Creates a new value under this reference.
// Returns a reference to the newly created value.
// https://www.firebase.com/docs/web/api/firebase/push.html
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

// Overwrites the value at the specified path and returns a reference
// that points to the path specified by `path`
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
// Returns an error if the update could not be performed.
// https://www.firebase.com/docs/web/api/firebase/update.html
func (c *Client) Update(path string, value interface{}, params map[string]string) error {
	err := c.api.Call("PATCH", c.Url+"/"+path, c.Auth, value, params, nil)
	return err
}

// Remove deletes the data at the current reference.
// https://www.firebase.com/docs/web/api/firebase/remove.html
func (c *Client) Remove(path string, params map[string]string) error {
	err := c.api.Call("DELETE", c.Url+"/"+path, c.Auth, nil, params, nil)

	return err
}

// Rules returns the security rules for the database.
// https://www.firebase.com/docs/rest/api/#section-security-rules
func (c *Client) Rules(params map[string]string) (*Rules, error) {
	res := &Rules{}
	err := c.api.Call("GET", c.Url+"/.settings/rules", c.Auth, nil, params, res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SetRules overwrites the existing security rules with the new rules given.
// https://www.firebase.com/docs/rest/api/#section-security-rules
func (c *Client) SetRules(rules *Rules, params map[string]string) error {
	err := c.api.Call("PUT", c.Url+"/.settings/rules", c.Auth, rules, params, nil)

	return err
}

// Call invokes the appropriate HTTP method on a given Firebase URL.
func (f *f) Call(method, path, auth string, body interface{}, params map[string]string, dest interface{}) error {

	// Every path needs to end in .json for the Firebase REST API
	path += ".json"
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
