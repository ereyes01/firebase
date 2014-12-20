Go Firebase
========

## Summary

Helper library for invoking the Firebase REST API.

## Installation

### Build

```sh
go get github.com/JustinTulloss/firebase
```

### Test

Edit the firebase_test.go file and set the ```testUrl``` and ```testKey``` variables to match your Firebase account.

Then run:
```sh
go test github.com/JustinTulloss/firebase...
```

## Usage

First import the package into your code:
```go
import (
    "github.com/JustinTulloss/firebase"
)
```

To use the client, initialize it and make requests similarly to the Firebase docs:
```go
fire := firebase.NewClient("https://<TBD>.firebase.com", "<optional authentication token>", nil)

n := &Name { First: "Jack", Last: "Sparrow" }
jack, err_ := fire.Child("users/jack").Set("name", n, nil)
```

Currently, the following methods are supported:
```go
Child(path)
Push(value)
Set(path, value)
Update(path, value)
Remove(path)
Value()
Rules()
SetRules(rules)
```

For more details about the Firebase APIs, see the [Firebase official documentation](https://www.firebase.com/docs/).
