package firebase

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onsi/ginkgo/reporters"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	testUrl  = ""
	testAuth = ""
)

type Name struct {
	First string `json:",omitempty"`
	Last  string `json:",omitempty"`
}

func nameAlloc() interface{} {
	return &Name{}
}

func fakeServer(handler http.Handler) (*httptest.Server, *client) {
	testServer := httptest.NewServer(handler)

	c := NewClient(testServer.URL, testAuth, nil)
	testClient, isClient := c.(*client)
	Expect(isClient).To(BeTrue())

	return testServer, testClient
}

var _ = Describe("Transforming client urls/queries", func() {
	var (
		c        *client
		isClient bool
		testURL  string = "https://who.cares.com"
	)

	BeforeEach(func() {
		c, isClient = NewClient(testURL, testAuth, nil).(*client)
		Expect(isClient).To(BeTrue())
	})

	It("Adds the child path to the returned client object", func() {
		child, isClient := c.Child("child").(*client)
		Expect(isClient).To(BeTrue())

		Expect(child.url).To(Equal(testURL + "/child"))
	})

	It("Sets a query string param to ask for a shallow object", func() {
		shallow, isClient := c.Shallow().(*client)
		Expect(isClient).To(BeTrue())

		Expect(shallow.params["shallow"]).To(Equal("true"))
	})

	It("Retrieves the key of a client", func() {
		Expect(c.Key()).To(Equal(""))
		Expect(c.Child("test").Key()).To(Equal("test"))
		Expect(c.Child("/a/b/c/d/e/f/g").Key()).To(Equal("g"))
	})
})

var _ = Describe("Manipulating values from firebase", func() {
	var (
		testResource Name
		testServer   *httptest.Server
		testClient   *client
		handler      func(w http.ResponseWriter, r *http.Request)
		stopChannel  chan bool
	)

	BeforeEach(func() {
		testResource = Name{First: "FirstName", Last: "LastName"}
	})

	JustBeforeEach(func() {
		testServer, testClient = fakeServer(http.HandlerFunc(handler))
		stopChannel = make(chan bool, 1)
	})

	AfterEach(func() {
		close(stopChannel)
		testServer.Close()
	})

	Context("Retrieving a value from firebase", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("GET"))
				fmt.Fprintln(w, `{"bru": "haha"}`)
			}
		})

		It("Retrieves the expected value from the resource path", func() {
			var r map[string]interface{}
			err := testClient.Child("").Value(&r)
			Expect(err).To(BeNil())

			Expect(len(r)).To(Equal(1))
			Expect(r["bru"]).To(Equal("haha"))
		})
	})

	Context("Pushing a new value to firebase", func() {
		var (
			pushedName string = "baloo"
		)

		BeforeEach(func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("POST"))

				var pushed Name
				defer r.Body.Close()

				decoder := json.NewDecoder(r.Body)
				err := decoder.Decode(&pushed)
				Expect(err).To(BeNil())
				Expect(pushed).To(Equal(testResource))

				fmt.Fprintf(w, `{"name": "%s"}`, pushedName)
			}
		})

		It("Pushes the new resource and returns a matching client", func() {
			name := &testResource

			response, err := testClient.Child("path").Push(name, nil)
			Expect(err).To(BeNil())

			responseClient, isClient := response.(*client)
			Expect(isClient).To(BeTrue())
			Expect(responseClient.url).To(Equal(testServer.URL + "/path/" +
				pushedName))
		})
	})

	Context("Setting an existing value in firebase", func() {
		var (
			newName Name   = Name{First: "NewFirst", Last: "NewLast"}
			setPath string = "set"
		)

		BeforeEach(func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("PUT"))

				var setValue Name
				defer r.Body.Close()

				decoder := json.NewDecoder(r.Body)
				err := decoder.Decode(&setValue)
				Expect(err).To(BeNil())
				Expect(setValue).To(Equal(newName))
			}
		})

		It("Overwrites the value of the existing resource", func() {
			response, err := testClient.Set(setPath, &newName, nil)
			Expect(err).To(BeNil())

			responseClient, isClient := response.(*client)
			Expect(isClient).To(BeTrue())
			Expect(responseClient.url).To(Equal(testServer.URL + "/" +
				setPath))
		})
	})

	Context("Update an existing value in firebase", func() {
		var (
			updatedName Name   = Name{Last: "NewLast"}
			updatePath  string = "update"
		)

		BeforeEach(func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("PATCH"))
				Expect(r.URL.String()).To(Equal("/" + updatePath + ".json"))

				var updateValue Name
				defer r.Body.Close()

				decoder := json.NewDecoder(r.Body)
				err := decoder.Decode(&updateValue)
				Expect(err).To(BeNil())
				Expect(updateValue).To(Equal(updatedName))
			}
		})

		It("Changes the value of the existing resource", func() {
			err := testClient.Update(updatePath, &updatedName, nil)
			Expect(err).To(BeNil())
		})
	})

	Context("Delete an existing value in firebase", func() {
		var (
			rmPath string = "update"
		)

		BeforeEach(func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("DELETE"))
				Expect(r.URL.String()).To(Equal("/" + rmPath + ".json"))
			}
		})

		It("Deletes the resource", func() {
			err := testClient.Remove(rmPath, nil)
			Expect(err).To(BeNil())
		})
	})

	Context("Reading the security rules", func() {
		var testRules Rules = make(map[string]interface{})

		BeforeEach(func() {
			testRules["rules"] = "anything goes"

			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.String()).To(Equal("/.settings/rules.json"))

				encoder := json.NewEncoder(w)
				err := encoder.Encode(testRules)
				Expect(err).To(BeNil())
			}
		})

		It("Retrieves the firebase's security rules", func() {
			rules, err := testClient.Rules(nil)
			Expect(err).To(BeNil())
			Expect(*rules).To(Equal(testRules))
		})
	})

	Context("Changing the firebase's security rules", func() {
		var newRules Rules

		BeforeEach(func() {
			newRules = Rules{
				"rules": map[string]interface{}{
					".read":  "auth.username == 'admin'",
					".write": "auth.username == 'admin'",
					"ordered": map[string]interface{}{
						".indexOn": []string{"First"},
						"kids": map[string]interface{}{
							".indexOn": []string{"Age"},
						},
					},
				},
			}

			handler = func(w http.ResponseWriter, r *http.Request) {
				var changedRules Rules
				Expect(r.Method).To(Equal("PUT"))
				Expect(r.URL.String()).To(Equal("/.settings/rules.json"))

				defer r.Body.Close()
				decoder := json.NewDecoder(r.Body)
				err := decoder.Decode(&changedRules)
				Expect(err).To(BeNil())
			}
		})

		It("Changes the security rules", func() {
			err := testClient.SetRules(&newRules, nil)
			Expect(err).To(BeNil())
		})
	})

	Context("Watching a resource", func() {
		var (
			events <-chan StreamEvent
			errs   <-chan error
			err    error
		)

		JustBeforeEach(func() {
			testClient = testClient.Child("").(*client)
		})

		AfterEach(func() {
			Eventually(events).Should(BeClosed())
			Eventually(errs).Should(BeClosed())
		})

		Context("When receiving a keep-alive event", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: keep-alive")
					fmt.Fprintln(w, "data: null")
				}
			})

			It("Ignores the event", func() {
				events, errs, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())
				Consistently(events).ShouldNot(Receive())
				Consistently(errs).ShouldNot(Receive())
			})
		})

		Context("When Firebase permissions block the watched location", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: cancel")
					fmt.Fprintln(w, "data: null")
				}
			})

			It("Receives an error from the error channel", func() {
				events, errs, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())
				Consistently(events).ShouldNot(Receive())
				Eventually(errs).Should(Receive(MatchError("Permission Denied")))
			})
		})

		Context("When Firebase revokes your auth token", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: auth_revoked")
					fmt.Fprintln(w, "data: null")
				}
			})

			It("Receives an error from the error channel", func() {
				events, errs, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())
				Consistently(events).ShouldNot(Receive())
				Eventually(errs).Should(Receive(MatchError("Auth Token Revoked")))
			})
		})

		Context("When a resource is patched", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: patch")
					fmt.Fprintln(w, `data: {"path": "1/2/3", "data": {"a":1}}`)
				}
			})

			It("Receives an event with the unmarshalled object", func() {
				type Widget struct {
					A int
				}

				expectedData := StreamData{
					Path:    "1/2/3",
					RawData: []byte(`{"a":1}`),
					Object:  Widget{A: 1},
				}

				unmarshaller := func(jsonText []byte) (interface{}, error) {
					var w Widget
					err := json.Unmarshal(jsonText, &w)
					return w, err
				}

				events, errs, err = testClient.Watch(unmarshaller, stopChannel)
				Expect(err).To(BeNil())

				event := <-events
				Expect(event.Event).To(Equal("patch"))
				Expect(event.Error).To(BeNil())
				Expect(*event.Data).To(Equal(expectedData))
				Consistently(errs).ShouldNot(Receive())
			})
		})

		Context("When a resource is put", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: put")
					fmt.Fprintln(w, `data: {"path": "1/2/3", "data": {"a":1}}`)
				}
			})

			It("Receives an event with the unmarshalled object", func() {
				type Widget struct {
					A int
				}

				expectedData := StreamData{
					Path:    "1/2/3",
					RawData: []byte(`{"a":1}`),
					Object:  Widget{A: 1},
				}

				unmarshaller := func(jsonText []byte) (interface{}, error) {
					var w Widget
					err := json.Unmarshal(jsonText, &w)
					return w, err
				}

				events, errs, err = testClient.Watch(unmarshaller, stopChannel)
				Expect(err).To(BeNil())

				event := <-events
				Expect(event.Event).To(Equal("put"))
				Expect(event.Error).To(BeNil())
				Expect(*event.Data).To(Equal(expectedData))
				Consistently(errs).ShouldNot(Receive())
			})
		})

		Context("When web server sends bad JSON (hopefully never!)", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: patch")
					fmt.Fprintln(w, "data: {you know I'm bad, I'm bad}")
				}
			})

			It("Sends an event with a non-fatal error", func() {
				events, errs, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())

				event := <-events
				Expect(event.Event).To(Equal("patch"))
				Expect(event.Error).To(HaveOccurred())
				Consistently(errs).ShouldNot(Receive())
			})
		})

		Context("When the unmarshaller fails", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: put")
					fmt.Fprintln(w, `data: {"path": "1/2/3", "data": {}}`)
				}
			})

			It("Returns an event with error, no unmarshalled object", func() {
				expectedData := StreamData{
					Path:    "1/2/3",
					RawData: []byte("{}"),
					Object:  nil,
				}

				unmarshaller := func(jsonText []byte) (interface{}, error) {
					return 10, errors.New("crash")
				}

				events, errs, err = testClient.Watch(unmarshaller, stopChannel)
				Expect(err).To(BeNil())

				event := <-events
				Expect(event.Event).To(Equal("put"))
				Expect(event.Error).To(MatchError("crash"))
				Expect(*event.Data).To(Equal(expectedData))
				Consistently(errs).ShouldNot(Receive())
			})
		})

		Context("When no unmarshaller is specified", func() {
			BeforeEach(func() {
				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: put")
					fmt.Fprintln(w, `data: {"path": "1/2/3", "data": {"a":1}}`)
				}
			})

			It("Unmarshals the event's payload into a map", func() {
				expectedData := StreamData{
					Path:    "1/2/3",
					RawData: []byte(`{"a":1}`),
					Object:  map[string]interface{}{"a": float64(1)},
				}

				events, errs, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())

				event := <-events
				Expect(event.Event).To(Equal("put"))
				Expect(event.Error).To(BeNil())
				Expect(*event.Data).To(BeEquivalentTo(expectedData))
				Consistently(errs).ShouldNot(Receive())
			})
		})
	})
})

var _ = Describe("Firebase timestamps", func() {
	It("Marshals a timestamp into ms since the epoch", func() {
		ts := Timestamp(time.Now())
		marshaled, err := json.Marshal(&ts)
		Expect(err).To(BeNil())

		unmarshaledTs := Timestamp{}
		err = json.Unmarshal(marshaled, &unmarshaledTs)
		Expect(err).To(BeNil())

		// Compare unix timestamps as we lose some fidelity in the nanoseconds
		Expect(time.Time(ts).Unix()).To(Equal(time.Time(unmarshaledTs).Unix()))
	})

	It("Marhsals a server-side timestamp", func() {
		text, err := json.Marshal(ServerTimestamp)
		Expect(err).To(BeNil())
		Expect(string(text)).To(Equal(`{".sv":"timestamp"}`))
	})
})

func TestFirebase(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Firebase Suite",
		[]Reporter{junitReporter})
}