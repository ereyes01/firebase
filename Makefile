$(GOPATH)/src/%:
	go get $*

mock_firebase/firebase.go: firebase.go | $(GOPATH)/src/code.google.com/p/gomock/mockgen $(GOPATH)/src/code.google.com/p/gomock/gomock
	mockgen -source $^ -imports ".=github.com/JustinTulloss/firebase" -destination $@
