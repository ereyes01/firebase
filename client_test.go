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

type Widget struct {
	A int
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

	It("Sets a field to order the results by", func() {
		orderByClient, isClient := c.OrderBy("field").(*client)
		Expect(isClient).To(BeTrue())

		expectedParams := map[string]string{
			"orderBy": `"field"`,
		}
		Expect(orderByClient.params).To(BeEquivalentTo(expectedParams))
	})

	It("Queries for records whose field == true", func() {
		equalClient, isClient := c.OrderBy("field").EqualTo(true).(*client)
		Expect(isClient).To(BeTrue())

		expectedParams := map[string]string{
			"orderBy": `"field"`,
			"equalTo": "true",
		}
		Expect(equalClient.params).To(BeEquivalentTo(expectedParams))
	})

	It("Queries ranges of fields", func() {
		rangeClient, isClient := c.OrderBy("field").StartAt(0).EndAt(5).(*client)
		Expect(isClient).To(BeTrue())

		expectedParams := map[string]string{
			"orderBy": `"field"`,
			"startAt": "0",
			"endAt":   "5",
		}
		Expect(rangeClient.params).To(BeEquivalentTo(expectedParams))
	})

	It("Limits query results to first 10 children", func() {
		limitClient, isClient := c.OrderBy("field").LimitToFirst(5).(*client)
		Expect(isClient).To(BeTrue())

		expectedParams := map[string]string{
			"orderBy":      `"field"`,
			"limitToFirst": "5",
		}
		Expect(limitClient.params).To(BeEquivalentTo(expectedParams))
	})

	It("Limits query results to last 10 children", func() {
		limitClient, isClient := c.OrderBy("field").LimitToLast(5).(*client)
		Expect(isClient).To(BeTrue())

		expectedParams := map[string]string{
			"orderBy":     `"field"`,
			"limitToLast": "5",
		}
		Expect(limitClient.params).To(BeEquivalentTo(expectedParams))
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
			err    error
		)

		JustBeforeEach(func() {
			testClient = testClient.Child("").(*client)
		})

		AfterEach(func() {
			Eventually(events).Should(BeClosed())
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
				events, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())
				Consistently(events).ShouldNot(Receive())
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

			It("Receives an event with an error", func() {
				events, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())

				expected := StreamEvent{
					Event:   "cancel",
					RawData: "null",
					Error:   errors.New("Permission Denied"),
				}

				Eventually(events).Should(Receive(BeEquivalentTo(expected)))
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

			It("Receives an event with an error", func() {
				events, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())

				expected := StreamEvent{
					Event:   "auth_revoked",
					RawData: "null",
					Error:   errors.New("Auth Token Revoked"),
				}

				Eventually(events).Should(Receive(BeEquivalentTo(expected)))
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
				expectedEvent := StreamEvent{
					Event:    "patch",
					RawData:  `{"path": "1/2/3", "data": {"a":1}}`,
					Path:     "1/2/3",
					Resource: Widget{A: 1},
				}

				unmarshaller := func(path string, data []byte) (interface{}, error) {
					var w Widget
					Expect(path).To(Equal("1/2/3"))
					err := json.Unmarshal(data, &w)
					return w, err
				}

				events, err = testClient.Watch(unmarshaller, stopChannel)
				Expect(err).To(BeNil())
				Eventually(events).Should(Receive(Equal(expectedEvent)))
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
				expectedEvent := StreamEvent{
					Event:    "put",
					Path:     "1/2/3",
					RawData:  `{"path": "1/2/3", "data": {"a":1}}`,
					Resource: Widget{A: 1},
				}

				unmarshaller := func(path string, data []byte) (interface{}, error) {
					var w Widget
					Expect(path).To(Equal("1/2/3"))
					err := json.Unmarshal(data, &w)
					return w, err
				}

				events, err = testClient.Watch(unmarshaller, stopChannel)
				Expect(err).To(BeNil())
				Eventually(events).Should(Receive(Equal(expectedEvent)))
			})
		})

		Context("When web server sends bad JSON (hopefully never!)", func() {
			var (
				badData   string = "{you know I'm bad, I'm bad}"
				jsonError error
			)

			BeforeEach(func() {
				var scratch map[string]interface{}
				jsonError = json.Unmarshal([]byte(badData), &scratch)

				handler = func(w http.ResponseWriter, r *http.Request) {
					verifyStreamRequest(r)

					fmt.Fprintln(w, "event: patch")
					fmt.Fprintln(w, "data: "+badData)
				}
			})

			It("Sends an event with a non-fatal error", func() {
				expectedEvent := StreamEvent{
					Event:   "patch",
					RawData: badData,
					Error:   jsonError,
				}

				events, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())
				Eventually(events).Should(Receive(Equal(expectedEvent)))
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
				expectedEvent := StreamEvent{
					Event:             "put",
					Path:              "1/2/3",
					RawData:           `{"path": "1/2/3", "data": {}}`,
					UnmarshallerError: errors.New("crash"),
				}

				unmarshaller := func(path string, data []byte) (interface{}, error) {
					Expect(path).To(Equal("1/2/3"))
					return 10, errors.New("crash")
				}

				events, err = testClient.Watch(unmarshaller, stopChannel)
				Expect(err).To(BeNil())
				Eventually(events).Should(Receive(Equal(expectedEvent)))
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
				expectedEvent := StreamEvent{
					Event:    "put",
					Path:     "1/2/3",
					RawData:  `{"path": "1/2/3", "data": {"a":1}}`,
					Resource: map[string]interface{}{"a": float64(1)},
				}

				events, err = testClient.Watch(nil, stopChannel)
				Expect(err).To(BeNil())
				Eventually(events).Should(Receive(Equal(expectedEvent)))
			})
		})
	})
})

var _ = Describe("Firebase timestamps", func() {
	It("Unmarshals a Firebase ms timestamp into a Go time type", func() {
		nowMs := (time.Now().UnixNano() / int64(time.Millisecond))
		ts := fmt.Sprint(nowMs)

		unmarshaledTs := ServerTimestamp{}
		err := json.Unmarshal([]byte(ts), &unmarshaledTs)
		Expect(err).To(BeNil())

		unmarshalledNs := time.Time(unmarshaledTs).UnixNano()
		Expect(nowMs * int64(time.Millisecond)).To(Equal(unmarshalledNs))
	})

	It("Marshals a server-side timestamp into a server-value", func() {
		var ts ServerTimestamp
		ts = ServerTimestamp(time.Now())

		text, err := json.Marshal(ts)
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
