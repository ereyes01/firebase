package firebase

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/facebookgo/httpcontrol"
)

// httpClient is the HTTP client used to make calls to Firebase with the default API
var httpClient = newTimeoutClient(connectTimeout, readWriteTimeout)

// streamClient is used as the HTTP client for streaming Event Source protocol
// messages from Firebase
var streamClient = newTimeoutClient(connectTimeout, streamTimeout)

// f is the internal implementation of the Firebase API client.
type firebaseAPI struct{}

var (
	connectTimeout   = time.Duration(30 * time.Second) // timeout for http connection
	readWriteTimeout = time.Duration(10 * time.Second) // timeout for http read/write
	streamTimeout    = time.Duration(0)                // never time out reading from a stream
)

func doFirebaseRequest(client *http.Client, method, path, auth, accept string, body interface{}, params map[string]string) (*http.Response, error) {
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

	return client.Do(req)
}

// Call invokes the appropriate HTTP method on a given Firebase URL.
func (f *firebaseAPI) Call(method, path, auth string, body interface{}, params map[string]string, dest interface{}) error {
	response, err := doFirebaseRequest(httpClient, method, path, auth, "",
		body, params)
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
			MaxTries:            30,
			MaxIdleConnsPerHost: 5,
		},
	}
}

// Stream implements an SSE/Event Source client that watches for changes at a
// given Firebase location.
func (f *firebaseAPI) Stream(path, auth string, body interface{}, params map[string]string, stop <-chan bool) (<-chan RawEvent, error) {
	response, err := doFirebaseRequest(streamClient, "GET", path, auth,
		"text/event-stream", body, params)
	if err != nil {
		return nil, err
	}

	go func() {
		<-stop
		response.Body.Close()
	}()

	events := make(chan RawEvent, 1000)

	// bufio.Scanner barfs on really long events, as its buffer size is pretty
	// small, and it can't be overridden. This is not the most memory-optimal
	// implementation of the streaming client, but each event is not expected
	// to exceed several KB.
	go func() {
		var (
			err       error
			firstLine string
			lineBuf   []byte
		)

		byteReader := bufio.NewReader(response.Body)

		for {
			var b byte

			b, err = byteReader.ReadByte()
			if err != nil {
				break
			}

			if b != "\n"[0] {
				lineBuf = append(lineBuf, b)
				continue
			}

			if firstLine == "" {
				firstLine = string(lineBuf)
				lineBuf = []byte{}
				continue
			}

			line := string(lineBuf)

			event := RawEvent{}
			event.Event = strings.Replace(firstLine, "event: ", "", 1)
			event.Data = strings.Replace(line, "data: ", "", 1)

			events <- event
			firstLine = ""
			lineBuf = []byte{}
		}

		if err == io.EOF {
			err = nil
		}

		closeEvent := RawEvent{Error: err}
		events <- closeEvent
		close(events)
	}()

	return events, nil
}
