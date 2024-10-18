.PHONY: clean test

tiarraview: clean go.* *.go
	go build --tags "fts5" -o $@ cmd/tiarraview/main.go

clean:
	rm -rf tiarraview dist/

test:
	go test -v ./...

install:
	go install github.com/fujiwara/tiarraview/cmd/tiarraview

dist:
	goreleaser build --snapshot --clean
