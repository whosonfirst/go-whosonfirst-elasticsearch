vuln:
	govulncheck ./...

cli:
	go build -mod vendor -o bin/wof-index-elasticsearch cmd/wof-index-elasticsearch/main.go
