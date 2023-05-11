GOMOD=vendor

vuln:
	govulncheck ./...

cli:
	go build -mod $(GOMOD) -ldflags="-s -w"  -o bin/wof-index-elasticsearch cmd/wof-index-elasticsearch/main.go
