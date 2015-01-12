package firebase_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/JustinTulloss/firebase"
)

type Name struct {
	First string `json:",omitempty"`
	Last  string `json:",omitempty"`
}

/*
Set the two variables below and set them to your own
Firebase URL and credentials (optional) if you're forking the code
and want to test your changes.
*/

// enter your firebase credentials for testing.
var (
	testUrl  string = os.Getenv("FIREBASE_TEST_URL")
	testAuth string = os.Getenv("FIREBASE_TEST_AUTH")
)

func TestValue(t *testing.T) {
	client := firebase.NewClient(testUrl+"/tests", testAuth, nil)

	var r map[string]interface{}
	err := client.Value(&r)
	if err != nil {
		t.Error(err)
	}

	if r == nil {
		t.Fatalf("No values returned from the server\n")
	}
}

func TestChild(t *testing.T) {
	client := firebase.NewClient(testUrl+"/tests", testAuth, nil)

	r := client.Child("")

	if r == nil {
		t.Fatalf("No child returned from the server\n")
	}
}

func TestPush(t *testing.T) {
	client := firebase.NewClient(testUrl+"/tests", testAuth, nil)

	name := &Name{First: "FirstName", Last: "LastName"}

	r, err := client.Push(name, nil)

	if err != nil {
		t.Fatalf("%v\n", err)
	}

	if r == nil {
		t.Fatalf("No client returned from the server\n")
	}

	newName := &Name{}
	c2 := firebase.NewClient(r.String(), testAuth, nil)
	c2.Value(newName)
	if !reflect.DeepEqual(name, newName) {
		t.Errorf("Expected %v to equal %v", name, newName)
	}
}

func TestSet(t *testing.T) {
	c1 := firebase.NewClient(testUrl+"/tests/users", testAuth, nil)

	name := &Name{First: "First", Last: "last"}
	c2, _ := c1.Push(name, nil)

	newName := &Name{First: "NewFirst", Last: "NewLast"}
	r, err := c2.Set("", newName, map[string]string{"print": "silent"})

	if err != nil {
		t.Fatalf("%v\n", err)
	}

	if r == nil {
		t.Fatalf("No client returned from the server\n")
	}
}

func TestUpdate(t *testing.T) {
	c1 := firebase.NewClient(testUrl+"/tests/users", testAuth, nil)

	name := &Name{First: "First", Last: "last"}
	c2, _ := c1.Push(name, nil)

	newName := &Name{Last: "NewLast"}
	err := c2.Update("", newName, nil)

	if err != nil {
		t.Fatalf("%v\n", err)
	}
}

func TestRemovet(t *testing.T) {
	c1 := firebase.NewClient(testUrl+"/tests/users", testAuth, nil)

	name := &Name{First: "First", Last: "last"}
	c2, _ := c1.Push(name, nil)

	err := c2.Remove("", nil)
	if err != nil {
		t.Fatalf("%v\n", err)
	}

	var val map[string]interface{}
	c3 := firebase.NewClient(c2.String(), testAuth, nil)
	err = c3.Value(&val)
	if err != nil {
		t.Error(err)
	}

	if len(val) != 0 {
		t.Errorf("Expected %s to be removed, was %v", c2.String(), val)
	}
}

func TestRules(t *testing.T) {
	client := firebase.NewClient(testUrl, testAuth, nil)

	r, err := client.Rules(nil)

	if err != nil {
		t.Fatalf("Error retrieving rules: %v\n", err)
	}

	if r == nil {
		t.Fatalf("No child returned from the server\n")
	}
}

func TestSetRules(t *testing.T) {
	client := firebase.NewClient(testUrl, testAuth, nil)

	rules := &firebase.Rules{
		"rules": map[string]string{
			".read":  "auth.username == 'admin'",
			".write": "auth.username == 'admin'",
		},
	}

	err := client.SetRules(rules, nil)

	if err != nil {
		t.Fatalf("Error retrieving rules: %v\n", err)
	}
}

func TestTimestamp(t *testing.T) {
	ts := firebase.Timestamp(time.Now())
	marshaled, err := json.Marshal(&ts)
	if err != nil {
		t.Fatalf("Could not marshal a timestamp to json: %s\n", err)
	}
	unmarshaledTs := firebase.Timestamp{}
	err = json.Unmarshal(marshaled, &unmarshaledTs)
	if err != nil {
		t.Fatalf("Could not unmarshal a timestamp to json: %s\n", err)
	}
	// Compare unix timestamps as we lose some fidelity in the nanoseconds
	if time.Time(ts).Unix() != time.Time(unmarshaledTs).Unix() {
		t.Fatalf("Unmarshaled time %s not equivalent to marshaled time %s",
			unmarshaledTs,
			ts,
		)
	}
}

func TestServerTimestamp(t *testing.T) {
	b, err := json.Marshal(firebase.ServerTimestamp)
	if err != nil {
		t.Fatalf("Could not marshal server timestamp: %s\n", err)
	}
	if string(b) != `{".sv":"timestamp"}` {
		t.Fatalf("Unexpected timestamp json value: %s\n", b)
	}
}

func TestIterator(t *testing.T) {
	client := firebase.NewClient(testUrl+"/test-iterator", testAuth, nil)
	defer client.Remove("", nil)
	names := []Name{
		{First: "FirstName", Last: "LastName"},
		{First: "Second", Last: "Seconder"},
	}
	for _, name := range names {
		_, err := client.Push(name, nil)
		if err != nil {
			t.Fatalf("%v\n", err)
		}
	}

	var i = 0
	for nameEntry := range client.Iterator(func() interface{} { return &Name{} }) {
		name := nameEntry.Value.(*Name)
		if !reflect.DeepEqual(&names[i], name) {
			t.Errorf("Expected %v to equal %v", &names[i], name)
		}
		i++
	}
	if i != len(names) {
		t.Fatalf("Did not receive all names, received %d\n", i)
	}
}

func TestMain(m *testing.M) {
	if testUrl == "" || testAuth == "" {
		fmt.Printf("You need to set FIREBASE_TEST_URL and FIREBASE_TEST_AUTH\n")
		os.Exit(1)
	}
	os.Exit(m.Run())
}
