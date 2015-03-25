package firebase

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/facebookgo/httpcontrol"
)

// httpClient is the HTTP client used to make calls to Firebase with the default API
var httpClient = newTimeoutClient(connectTimeout, readWriteTimeout)

// f is the internal implementation of the Firebase API client.
type firebaseAPI struct{}

var (
	connectTimeout   = time.Duration(30 * time.Second) // timeout for http connection
	readWriteTimeout = time.Duration(10 * time.Second) // timeout for http read/write
)

func doFirebaseRequest(method, path, auth, accept string, body interface{}, params map[string]string) (*http.Response, error) {
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
		return nil, err
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(encodedBody))
	if err != nil {
		return nil, err
	}

	if accept != "" {
		req.Header.Add("Accept", accept)
	}

	req.Close = true

	return httpClient.Do(req)
}

// Call invokes the appropriate HTTP method on a given Firebase URL.
func (f *firebaseAPI) Call(method, path, auth string, body interface{}, params map[string]string, dest interface{}) error {
	response, err := doFirebaseRequest(method, path, auth, "", body, params)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	decoder := json.NewDecoder(response.Body)
	if response.StatusCode >= 400 {
		err := &FirebaseError{}
		decoder.Decode(err)
		return err
	}

	if dest != nil && response.ContentLength != 0 {
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
			RequestTimeout:      readWriteTimeout,
			DialTimeout:         connectTimeout,
			MaxTries:            3,
			MaxIdleConnsPerHost: 200,
		},
	}
}

// Stream implements an SSE/Event Source client that watches for changes at a
// given Firebase location.
func (f *firebaseAPI) Stream(path, auth string, body interface{}, params map[string]string, stop <-chan bool) (<-chan StreamEvent, error) {
	response, err := doFirebaseRequest("GET", path, auth, "text/event-stream",
		body, params)
	if err != nil {
		return nil, err
	}

	go func() {
		<-stop
		response.Body.Close()
	}()

	events := make(chan StreamEvent, 100)

	go func() {
		var err error

		defer func() {
			closedEvent := StreamEvent{Error: err}
			events <- closedEvent
			close(events)
		}()

		scanner := bufio.NewScanner(response.Body)
		firstLine := ""

		for scanner.Scan() {
			line := scanner.Text()

			// we want to process 2 lines at a time
			if firstLine == "" {
				firstLine = line
				continue
			}

			event := StreamEvent{}
			event.Event = strings.Replace(firstLine, "event: ", "", 1)

			data := strings.Replace(line, "data: ", "", 1)
			event.Error = json.Unmarshal([]byte(data), &event.Data)

			events <- event
			firstLine = ""
		}

		err = scanner.Err()
	}()

	return events, nil
}
