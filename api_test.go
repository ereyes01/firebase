package firebase

import (
	"fmt"
	"net/http"
	"net/http/httptest"

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
