package firebase

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func verifyStreamRequest(request *http.Request) {
	Expect(request.Method).To(Equal("GET"))
	Expect(request.Header.Get("Accept")).To(Equal("text/event-stream"))
}

var _ = Describe("Firebase SSE/Event Source client", func() {
	var (
		testServer  *httptest.Server
		testClient  *client
		testAPI     Api
		handler     func(w http.ResponseWriter, r *http.Request)
		nullHandler func(w http.ResponseWriter, r *http.Request)
		stopChannel chan bool
	)

	BeforeEach(func() {
		nullHandler = func(w http.ResponseWriter, r *http.Request) {
			verifyStreamRequest(r)
			// no events, just terminate the session
		}
	})

	JustBeforeEach(func() {
		testServer, testClient = fakeServer(http.HandlerFunc(handler))
		testClient = testClient.Child("").(*client)
		testAPI = testClient.api
		stopChannel = make(chan bool)
	})

	AfterEach(func() {
		close(stopChannel)
		testServer.Close()
	})

	Context("When the connection terminates with EOF", func() {
		BeforeEach(func() {
			handler = nullHandler
		})

		It("Receives an empty event", func() {
			events, err := testAPI.Stream(testClient.url, testAuth, nil, nil,
				stopChannel)
			Expect(err).To(BeNil())
			Eventually(events).Should(Receive(Equal(RawEvent{})))
		})
	})

	Context("Handling a single event", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				verifyStreamRequest(r)

				fmt.Fprintln(w, "event: hi")
				fmt.Fprintln(w, "data: there")
			}
		})

		It("Fires a single event", func() {
			expectedEvent := RawEvent{Event: "hi", Data: "there"}

			events, err := testAPI.Stream(testClient.url, testAuth, nil, nil,
				stopChannel)
			Expect(err).To(BeNil())

			Eventually(events).Should(Receive(Equal(expectedEvent)))
		})
	})

	Context("Handling two events", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				verifyStreamRequest(r)

				fmt.Fprintln(w, "event: hi")
				fmt.Fprintln(w, "data: there")
				fmt.Fprintf(w, "\n")
				fmt.Fprintln(w, "event: hey")
				fmt.Fprintln(w, "data: you")
			}
		})

		It("Fires two events", func() {
			expectedEvent1 := RawEvent{Event: "hi", Data: "there"}
			expectedEvent2 := RawEvent{Event: "hey", Data: "you"}

			events, err := testAPI.Stream(testClient.url, testAuth, nil, nil,
				stopChannel)
			Expect(err).To(BeNil())
			Eventually(events).Should(Receive(Equal(expectedEvent1)))
			Eventually(events).Should(Receive(Equal(expectedEvent2)))
		})
	})
})

var _ = Describe("Parsing timeouts / tunables from env variables", func() {
	var (
		testVariable    = "FIREBASE_TIMEOUT_TEST"
		defaultDuration = time.Duration(time.Second)
		defaultTunable  = 1
	)

	AfterEach(func() {
		os.Setenv(testVariable, "")
	})

	Context("Custom timeout specified via environment variable", func() {
		var (
			testTimeout      = "5m7s"
			expectedDuration = time.Duration(5*time.Minute + 7*time.Second)
		)

		BeforeEach(func() {
			os.Setenv(testVariable, testTimeout)
		})

		It("Returns the correct duration from an environment variable", func() {
			amount := parseTimeout(testVariable, defaultDuration)
			Expect(amount).To(Equal(expectedDuration))
		})
	})

	Context("No custom timeout specified via environment variable", func() {
		BeforeEach(func() {
			os.Setenv(testVariable, "")
		})

		It("Returns the default duration", func() {
			amount := parseTimeout(testVariable, defaultDuration)
			Expect(amount).To(Equal(defaultDuration))
		})
	})

	Context("Unparsable timeout specified via environment variable", func() {
		BeforeEach(func() {
			os.Setenv(testVariable, "zzzz")
		})

		It("Returns the default duration", func() {
			amount := parseTimeout(testVariable, defaultDuration)
			Expect(amount).To(Equal(defaultDuration))
		})
	})

	Context("Custom tunable specified via env variable", func() {
		var (
			testTunable   = "10"
			expectedValue = 10
		)

		BeforeEach(func() {
			os.Setenv(testVariable, testTunable)
		})

		It("Returns the correct tunable from an environment variable", func() {
			tunable := parseTunable(testVariable, defaultTunable)
			Expect(tunable).To(Equal(expectedValue))
		})
	})

	Context("No custom tunable specified via env variable", func() {
		BeforeEach(func() {
			os.Setenv(testVariable, "")
		})

		It("Returns the default tunable", func() {
			tunable := parseTunable(testVariable, defaultTunable)
			Expect(tunable).To(Equal(defaultTunable))
		})
	})

	Context("Unparsable tunable specified via env variable", func() {
		BeforeEach(func() {
			os.Setenv(testVariable, "zzzz")
		})

		It("Returns the default tunable", func() {
			tunable := parseTunable(testVariable, defaultTunable)
			Expect(tunable).To(Equal(defaultTunable))
		})
	})
})
