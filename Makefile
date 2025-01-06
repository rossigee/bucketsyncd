build: linux macos windows

linux:
	GOOS=linux GOARCH=amd64 go build -o build/bucketsyncd
macos:
	GOOS=darwin GOARCH=amd64 go build -o build/bucketsyncd.app
windows:
	GOOS=windows GOARCH=amd64 go build -o build/bucketsyncd.exe

