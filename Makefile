build: linux macos windows

linux:
	GOOS=linux GOARCH=amd64 go build -o s3sync main.go
macos:
	GOOS=darwin GOARCH=amd64 go build -o s3sync.app main.go
windows:
	GOOS=windows GOARCH=amd64 go build -o s3sync.exe main.go

