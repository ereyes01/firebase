Go Firebase
===========

Helper library for invoking the Firebase REST API. Supports the following operations:
- Read/Set/Push values to a Firebase path
- Streaming updates to a Firebase path via the SSE / Event Source protocol
- Firebase-friendly timestamp handling

Based on the great work of [cosn](https://github.com/cosn) and [JustinTulloss](https://github.com/JustinTulloss).

### Build Status

[![Circle CI](https://circleci.com/gh/ereyes01/firebase.svg?style=svg)](https://circleci.com/gh/ereyes01/firebase)

### Usage

[![GoDoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/ereyes01/firebase)

### Installation

- Setup your GOPATH and workspace. If you are new to Go and you're not sure how
to do this, read [How to Write Go Code](https://golang.org/doc/code.html).
- Dowload the package:
```sh
go get github.com/ereyes01/firebase
```

### Run the Tests

- Install test dependencies (only needed once per workspace):
```sh
go get -t github.com/ereyes01/firebase
```
- To run the tests:
```sh
go test github.com/ereyes01/firebase
```
