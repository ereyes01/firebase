package firebase

import (
	"encoding/json"
)

// Rules is the structure for security rules.
type Rules map[string]interface{}

// EventUnmarshaller callback accepts a raw JSON string and unmarshals it to
// any type of the implementor's choosing. The unmarshalled object is returned
// as an interface{}, or an error is returned if the unmarshal fails.
type EventUnmarshaller func(jsonText []byte) (interface{}, error)

type Client interface {
	// Returns the absolute URL path for the client
	String() string

	// Returns the last part of the URL path for the client.
	Key() string

	//Gets the value referenced by the client and unmarshals it into
	// the passed in destination.
	Value(destination interface{}) error

	// Watch streams changes to the Client's path in real-time, in a separate
	// goroutine.
	//
	// Arguments
	//
	// unmarshaller: Responsible for unmarshalling each resource change event's
	// payload into the desired type. If unmarshaller is nil, each event will
	// be unmarshalled into a map[string]interface{} object.
	//
	// stop: Sending any boolean value to this channel will stop the Watching
	// the client's path.
	//
	// Return Values
	//
	// <-chan StreamEvent - A channel that sends each received event.
	//
	// <-chan error - A channel that sends a fatal error that causes the Watch
	// goroutine to terminate.
	//
	// error - If non-nil, a fatal error was encountered starting the Watch
	// goroutine.
	Watch(unmarshaller EventUnmarshaller, stop <-chan bool) (<-chan StreamEvent, <-chan error, error)

	// Shallow returns a list of keys at a particular location
	// Only supports objects, unlike the REST artument which supports
	// literals. If the location is a literal, use Client#Value()
	Shallow() Client

	// Child returns a reference to the child specified by `path`. This does not
	// actually make a request to firebase, but you can then manipulate the reference
	// by calling one of the other methods (such as `Value`, `Update`, or `Set`).
	Child(path string) Client

	// Query functions. They map directly to the Firebase operations.
	// https://www.firebase.com/docs/rest/guide/retrieving-data.html#section-rest-queries
	OrderBy(prop string) Client
	EqualTo(value string) Client
	StartAt(value string) Client
	EndAt(value string) Client

	// Creates a new value under this reference.
	// Returns a reference to the newly created value.
	// https://www.firebase.com/docs/web/api/firebase/push.html
	Push(value interface{}, params map[string]string) (Client, error)

	// Overwrites the value at the specified path and returns a reference
	// that points to the path specified by `path`
	Set(path string, value interface{}, params map[string]string) (Client, error)

	// Update performs a partial update with the given value at the specified path.
	// Returns an error if the update could not be performed.
	// https://www.firebase.com/docs/web/api/firebase/update.html
	Update(path string, value interface{}, params map[string]string) error

	// Remove deletes the data at the current reference.
	// https://www.firebase.com/docs/web/api/firebase/remove.html
	Remove(path string, params map[string]string) error

	// Rules returns the security rules for the database.
	// https://www.firebase.com/docs/rest/api/#section-security-rules
	Rules(params map[string]string) (*Rules, error)

	// SetRules overwrites the existing security rules with the new rules given.
	// https://www.firebase.com/docs/rest/api/#section-security-rules
	SetRules(rules *Rules, params map[string]string) error
}

// StreamData represents the unmarshalled JSON payload of each EventSource
// protocol message
type StreamData struct {
	// The Firebase Path that was changed
	Path string

	// The raw JSON data that was changed within the path
	RawData json.RawMessage `json:"data"`

	// The unmarshalled data
	Object interface{}
}

// StreamEvent represents an EventSource protocol message sent by Firebase when
// streaming changes from a location.
type StreamEvent struct {
	// Event is the type of event, denoted in the protocol by "event: text"
	Event string

	// Data is the payload of the event, denoted in the protocol by "data: text"
	Data *StreamData

	// Contains a non-fatal error encountered in the processing of an event
	Error error
}

// Api is the internal interface for interacting with Firebase. The internal
// implementation of this interface is responsible for all HTTP operations that
// communicate with Firebase.
//
// Users of this library can implement their own Api-conformant types for
// testing purposes. To use your own test Api type, pass it in to the NewClient
// function.
type Api interface {
	// Call is responsible for performing HTTP transactions such as GET, POST,
	// PUT, PATCH, and DELETE. It is used to communicate with Firebase by all
	// of the Client methods, except for Watch.
	//
	// Arguments are as follows:
	//  - `method`: The http method for this call
	//  - `path`: The full firebase url to call
	//  - `body`: Data to be marshalled to JSON (it's the responsibility of Call to do the marshalling and unmarshalling)
	//  - `params`: Additional parameters to be passed to firebase
	//  - `dest`: The object to save the unmarshalled response body to.
	//    It's up to this method to unmarshal correctly, the default implemenation just uses `json.Unmarshal`
	Call(method, path, auth string, body interface{}, params map[string]string, dest interface{}) error

	// Stream is responsible for implementing a SSE/Event Source client that
	// communicates with Firebase to watch changes to a location in real-time.
	//
	// Arguments are as follows:
	//  - `path`: The full firebase url to call
	//  - `body`: Data to be marshalled to JSON
	//  - `params`: Additional parameters to be passed to firebase
	//  - `stop`: a channel that makes Stream stop listening for events and return when it receives anything
	//
	// Return values:
	//  - `<-StreamEvent`: A channels that emits events as they arrive from the stream
	//  - `error`: Non-nil if an error is encountered setting up the listener.
	Stream(path, auth string, body interface{}, params map[string]string, stop <-chan bool) (<-chan StreamEvent, error)
}
