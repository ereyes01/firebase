package firebase

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/facebookgo/httpcontrol"
)

var (
	// connectTimeoutDefault is the default timeout for regular http connection
	connectTimeoutDefault = time.Duration(300 * time.Second)

	// readWriteTimeoutDefaul is the default timeout for http read/write
	readWriteTimeoutDefault = time.Duration(100 * time.Second)

	// streamTimeoutDefault is the default timeout for streaming http clients.
	// By default, never time out reading from a stream
	streamTimeoutDefault = time.Duration(0)

	// maxTriesDefault is the default number of times a connection to Firebase
	// will be retried by the httpcontrol library.
	maxTriesDefault = 300

	// maxIdleConnsDefault is the default maximum number of idle connections to
	// Firebase that the httpcontrol library will allow.
	maxIdleConnsDefault = 30

	// httpClient is the connection pool for regular short lived HTTP calls to
	// Firebase.
	httpClient *http.Client

	// streamClient is the connection pool for long lived Event Source / SSE
	// stream connections to Firebase.
	streamClient *http.Client
)

func newTimeoutClient(connectTimeout, readWriteTimeout time.Duration, maxTries, maxIdleConnsPerHost int) *http.Client {
	return &http.Client{
		Transport: &httpcontrol.Transport{
			RequestTimeout:      readWriteTimeout,
			DialTimeout:         connectTimeout,
			MaxTries:            uint(maxTries),
			RetryAfterTimeout:   true,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
		},
	}
}

func parseTimeout(envVariableName string, defaultTimeout time.Duration) time.Duration {
	if timeout := os.Getenv(envVariableName); timeout != "" {
		if timeoutDuration, err := time.ParseDuration(timeout); err == nil {
			return timeoutDuration
		}
	}

	return defaultTimeout
}

func parseTunable(envVariableName string, defaultTunable int) int {
	if tunableStr := os.Getenv(envVariableName); tunableStr != "" {
		if tunable, err := strconv.Atoi(tunableStr); err == nil {
			return tunable
		}
	}

	return defaultTunable
}

func init() {
	connectTimeout := parseTimeout("FIREBASE_CONNECT_TIMEOUT",
		connectTimeoutDefault)
	readWriteTimeout := parseTimeout("FIREBASE_READWRITE_TIMEOUT",
		readWriteTimeoutDefault)
	streamTimeout := parseTimeout("FIREBASE_STREAM_TIMEOUT",
		streamTimeoutDefault)

	maxTries := parseTunable("FIREBASE_MAXTRIES", maxTriesDefault)
	maxIdleConnsPerHost := parseTunable("FIREBASE_MAXIDLE", maxIdleConnsDefault)

	httpClient = newTimeoutClient(connectTimeout, readWriteTimeout, maxTries,
		maxIdleConnsPerHost)
	streamClient = newTimeoutClient(connectTimeout, streamTimeout, maxTries,
		maxIdleConnsPerHost)
}
