Go Firebase
===========

**NOTE**: This repo is in *maintenance mode*. If you're developing a new application, you should take a look at Google's [official Firebase bindings](https://github.com/firebase/firebase-admin-go). As of June 2020, this code is still stable and being used in production for real products. Maintenance / bug fixing will still be done if any is needed.

Helper library for invoking the Firebase REST API from your Go program. Supports the
following operations:
- Read and write values using Firebase's REST API operations
- Stream updates to a Firebase path via the SSE / Event Source protocol
- Use native Go types/structs in all Firebase operations
- Server-side timestamps that are automatically converted into native Go times
- Read and modify security rules

My starting point was the great work of [cosn](https://github.com/cosn/firebase) and 
[JustinTulloss](https://github.com/JustinTulloss/firebase). Most of the code has since
been refactored, and comprehensive unit tests that do not call out to the network have
been added. Also, support for streaming via SSE / Event Source from Firebase has been
added.

Please star on Github if you find this library useful! Thanks!

### Build Status

[![Circle CI](https://circleci.com/gh/ereyes01/firebase.svg?style=svg)](https://circleci.com/gh/ereyes01/firebase)

### Reference Documentation

[![GoDoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/ereyes01/firebase)

### Installation

- Setup your GOPATH and workspace. If you are new to Go and you're not sure how
to do this, read [How to Write Go Code](https://golang.org/doc/code.html).
- Dowload the package:
```sh
go get -u -t github.com/ereyes01/firebase
```

### Run the Tests

- To run the tests:
```sh
go test -race github.com/ereyes01/firebase
```

### Usage

The usage examples below will use the sample 
[Dinosaur Facts Firebase](https://dinosaur-facts.firebaseio.com/) used in the
[REST tutorial](https://www.firebase.com/docs/rest/guide/retrieving-data.html#section-rest-filtering)

```go
client := firebase.NewClient("https://dinosaur-facts.firebaseio.com", "", nil)
```

Suppose we have a struct defined that matches each entry in the `dinosaurs/` path of
this firebase. Our struct might be declared as follows:

```go
type Dinosaur struct {
	Appeared int
	Height   float64
	Length   float64
	Order    string
	Vanished int
	Weight   float64
}
```

We could retrieve the lambeosarus record as follows:

```go
var dino Dinosaur

err := client.Child("dinosaurs/lambeosaurus").Value(&dino)
```

We could query dinosaurs whose scores are greater than 50 as follows:

```go
dinoScores := make(map[string]int)

err := client.Child("scores").OrderBy("$value").StartAt(50).Value(&dinoScores)
```

If I wanted to create a new dinosaur score (NOTE: the permissions of this firebase do
not allow this), we could try:

```go
value, err := client.Child("scores").Set("velociraptor", 500, nil)
```

We of course, don't have permissions to write to this Firebase. The error you'd get back
should be:

```
Permission denied
```

Create your own free test Firebase and feel free to experiment with writing values!

Now suppose we wanted to watch changes to the Triceratops dinosaur in real-time. This,
of course, will be a boring example because Triceratops will probably never change.
However, this sample demonstrates how you would stream changes to a Firebase path in 
real-time (and stops streaming after 10 seconds):

```go
    stop := make(chan bool)
	go func() {
		<-time.After(10 * time.Second)
		close(stop)
	}()

    // this helps convert incoming events into Dinosaur objects
    dinoParser := func(path string, data []byte) (interface{}, error) {
		var dino *Dinosaur
		err := json.Unmarshal(data, &dino)
		return dino, err
	}

	events, err := client.Child("dinosaurs/triceratops").Watch(dinoParser, stop)
	if err != nil {
		log.Fatal(err)
	}

	for event := range events {
		if event.Error != nil {
			log.Println("Stream error:", event.Error)
            continue
		}

		if event.UnmarshallerError != nil {
			log.Println("Malformed event:" event.UnmarshallerError)
            continue
		}

		newTriceratops := event.Resource.(*Dinosaur)
	}
```

The code above will yield a stream of Go Dinosaur objects (or rather, pointers to them).
The magic is in the dinoParser callback. This function (passed to Watch) tells the watcher
how to parse the json payload of the incoming events- in this case as Dinosaur pointers.
When the streaming connection is closed, the events channel also closes.

When you watch a Firebase location, you'll get back an initial event showing the state of
the location as it was when you started watching it. Thereafter, you will receive an
event when a change happens that matches your criteria, or when some place in the
location _stopped_ matching your criteria. This can be a little confusing at first,
especially when you combine queries with watching resources. It's just the way Firebase
watching of resources works.

You can read more about this behavior in my [Stack Overflow question](http://stackoverflow.com/questions/29265457/does-firebase-rest-streaming-support-ordering-and-filtering-child-nodes) and the subsequent discussion with one of the Firebase dudes.

Please see the [Godoc reference](https://godoc.org/github.com/ereyes01/firebase) for a
guide to the code and a more detailed list of operations. Also, please familiarize
yourself with [Firebase's REST API capabilities](https://www.firebase.com/docs/rest/api/)
before trying anything with this library.

Please open issues for any bugs or suggestions you may have, or send me a PR. Thanks!
