// Package firebase gives a thin wrapper around the firebase REST API. It tries
// to mirror the Official firebase API somewhat closely. https://www.firebase.com/docs/web/api/
package firebase

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var keyExtractor = regexp.MustCompile(`https://.*/([^/]+)/?$`)

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

type ServerValue struct {
	Value string `json:".sv"`
}

type FirebaseError struct {
	Message string `json:"error"`
}

func (f *FirebaseError) Error() string {
	return f.Message
}

// Use this value to represent a Firebase server timestamp in a data structure.
// This should be used when you're sending data to Firebase, as opposed to
// the Timestamp type.
var ServerTimestamp ServerValue = ServerValue{"timestamp"}

// This is the actual default implementation
type client struct {
	// The ordering being enforced on this client
	Order string
	// url is the client's base URL used for all calls.
	url string

	// auth is authentication token used when making calls.
	// The token is optional and can also be overwritten on an individual
	// call basis via params.
	auth string

	// api is the underlying client used to make calls.
	api Api

	params map[string]string
}

func NewClient(root, auth string, api Api) Client {
	if api == nil {
		api = new(firebaseAPI)
	}

	return &client{url: root, auth: auth, api: api}
}

func (c *client) String() string {
	return c.url
}

func (c *client) Key() string {
	matches := keyExtractor.FindAllStringSubmatch(c.url, 1)
	// This is kind of an error. There should always be a / somewhere,
	// but if you just have the raw domain you don't really need one. So
	// we assume this is the case and return ""
	if len(matches) == 0 {
		return ""
	}
	return matches[0][1]
}

func (c *client) Value(destination interface{}) error {
	err := c.api.Call("GET", c.url, c.auth, nil, c.params, destination)
	if err != nil {
		return err
	}
	return nil
}

var defaultUnmarshaller = func(jsonText []byte) (interface{}, error) {
	var object map[string]interface{}
	err := json.Unmarshal(jsonText, &object)
	return object, err
}

func handlePatchPut(event *StreamEvent, unmarshaller EventUnmarshaller) {
	var halfParsedData struct {
		Path string
		Data json.RawMessage
	}

	err := json.Unmarshal([]byte(event.RawData), &halfParsedData)
	if err != nil {
		event.Error = err
		return
	}

	event.Path = halfParsedData.Path

	object, err := unmarshaller(halfParsedData.Data)
	if err != nil {
		event.UnmarshallerError = err
	} else {
		event.Resource = object
	}
}

func (c *client) Watch(unmarshaller EventUnmarshaller, stop <-chan bool) (<-chan StreamEvent, error) {
	rawEvents, err := c.api.Stream(c.url, c.auth, nil, c.params, stop)
	if err != nil {
		return nil, err
	}

	processedEvents := make(chan StreamEvent, 1000)

	if unmarshaller == nil {
		unmarshaller = defaultUnmarshaller
	}

	go func() {
		for rawEvent := range rawEvents {
			event := StreamEvent{
				Event:   rawEvent.Event,
				RawData: rawEvent.Data,
				Error:   rawEvent.Error,
			}

			// connection error: just forward it along
			if event.Error != nil {
				processedEvents <- event
				continue
			}

			switch event.Event {
			case "patch", "put":
				handlePatchPut(&event, unmarshaller)
				processedEvents <- event
			case "keep-alive":
				break
			case "cancel":
				event.Error = errors.New("Permission Denied")
				processedEvents <- event
			case "auth_revoked":
				event.Error = errors.New("Auth Token Revoked")
				processedEvents <- event
				close(processedEvents)
				return
			}
		}

		close(processedEvents)
	}()

	return processedEvents, nil
}

func (c *client) Shallow() Client {
	newParams := make(map[string]string)
	for key, value := range c.params {
		newParams[key] = value
	}
	newParams["shallow"] = "true"

	return &client{
		api:    c.api,
		auth:   c.auth,
		url:    c.url,
		params: newParams,
	}
}

func (c *client) Child(path string) Client {
	u := c.url + "/" + path
	return &client{
		api:    c.api,
		auth:   c.auth,
		url:    u,
		params: c.params,
	}
}

const (
	KeyProp = "$key"
)

// These are some shenanigans, golang. Shenanigans I say.
func (c *client) newParamMap(key, value string) map[string]string {
	ret := make(map[string]string, len(c.params)+1)
	for key, value := range c.params {
		ret[key] = value
	}
	jsonVal, _ := json.Marshal(value)
	ret[key] = string(jsonVal)
	return ret
}

func (c *client) clientWithNewParam(key, value string) *client {
	return &client{
		api:    c.api,
		auth:   c.auth,
		url:    c.url,
		params: c.newParamMap(key, value),
	}
}

// Query functions. They map directly to the Firebase operations.
// https://www.firebase.com/docs/rest/guide/retrieving-data.html#section-rest-queries
func (c *client) OrderBy(prop string) Client {
	newC := c.clientWithNewParam("orderBy", prop)
	newC.Order = prop
	return newC
}

func (c *client) EqualTo(value string) Client {
	return c.clientWithNewParam("equalTo", value)
}

func (c *client) StartAt(value string) Client {
	return c.clientWithNewParam("startAt", value)
}

func (c *client) EndAt(value string) Client {
	return c.clientWithNewParam("endAt", value)
}

func (c *client) Push(value interface{}, params map[string]string) (Client, error) {
	res := map[string]string{}
	err := c.api.Call("POST", c.url, c.auth, value, params, &res)
	if err != nil {
		return nil, err
	}

	return &client{
		api:    c.api,
		auth:   c.auth,
		url:    c.url + "/" + res["name"],
		params: c.params,
	}, nil
}

func (c *client) Set(path string, value interface{}, params map[string]string) (Client, error) {
	u := c.url + "/" + path

	err := c.api.Call("PUT", u, c.auth, value, params, nil)
	if err != nil {
		return nil, err
	}

	return &client{
		api:    c.api,
		auth:   c.auth,
		url:    u,
		params: c.params,
	}, nil
}

func (c *client) Update(path string, value interface{}, params map[string]string) error {
	err := c.api.Call("PATCH", c.url+"/"+path, c.auth, value, params, nil)
	return err
}

func (c *client) Remove(path string, params map[string]string) error {
	err := c.api.Call("DELETE", c.url+"/"+path, c.auth, nil, params, nil)

	return err
}

func (c *client) Rules(params map[string]string) (*Rules, error) {
	res := &Rules{}
	err := c.api.Call("GET", c.url+"/.settings/rules", c.auth, nil, params, res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *client) SetRules(rules *Rules, params map[string]string) error {
	err := c.api.Call("PUT", c.url+"/.settings/rules", c.auth, rules, params, nil)

	return err
}
