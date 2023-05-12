# go-elasticsearch-whosonfirst

Go package for indexing Who's On First records in Elasticsearch.

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/whosonfirst/go-whosonfirst-elasticsearch.svg)](https://pkg.go.dev/github.com/whosonfirst/go-whosonfirst-elasticsearch)

## Tools

To build binary versions of these tools run the `cli` Makefile target. For example:

```
$> make cli
go build -mod vendor -ldflags="-s -w"  -o bin/wof-index-elasticsearch cmd/wof-index-elasticsearch/main.go
```

### es-whosonfirst-index

```
$> ./bin/es-whosonfirst-index -h
  -iterator-uri string
    	A valid whosonfirst/go-whosonfirst-iterate/v2 URI. (default "repo://")
  -monitor-uri string
    	A valid sfomuseum/go-timings URI. (default "counter://PT60S")
  -writer-uri value
    	One or more valid whosonfirst/go-writer/v2 URIs, each encoded as a gocloud.dev/runtimevar URI.
```	

For example, assuming a `whosonfirst/go-writer/v2` URI of "elasticsearch://localhost:9200/whosonfirst":

```
$> bin/es-whosonfirst-index \
   	-writer-uri 'constant://?val=elasticsearch%3A%2F%2Flocalhost%3A9200%2Fwhosonfirst'
	/usr/local/data/whosonfirst-data-admin-ca
```

This command will publish (or write) all the records in `whosonfirst-data-admin-ca` to the `whosonfirst` Elasticsearch index.

The `es-whosonfirst-index` tool is a thin wrapper around the `iterwriter` tool which is provided by the [whosonfirst/go-whosonfirst-iterwriter](https://github.com/whosonfirst/go-whosonfirst-iterwriter) package.

The need to URL-encode `whosonfirst/go-writer/v2` URIs is unfortunate but is tolerated since the use to `gocloud.dev/runtimevar` URIs provides a means to keep secrets and other sensitive values out of command-line arguments (and by extension process lists). This may not be much of an issues for things like Elasticsearch but is an issue for things like MySQL DSN strings.

### Environment variables

You can set (or override) command line flags with environment variables. Environment variable are expected to:

* Be upper-cased
* Replace all instances of `-` with `_`
* Be prefixed with `WOF_`

For example the `-writer-uri` flag would be overridden by the `WOF_WRITER_URI` environment variable.

## Elasticsearch "writer" URIs

This code assumes Elasticsearch 7.x by default although this may change with future releases. It is possible to specify the use of Elasticsearch 8.x, 7.x or 2.x for indexing as follows:

### Default (Elasticsearch 7.x)

```
elasticsearch://{ES_ENDPOINT}/{ES_INDEX}
```

### Elasticsearch 8.x

```
elasticsearch7://{USER}:{PASSWORD}@{ES_ENDPOINT}/{ES_INDEX}?ca-cert-uri={CA_CERTIFICATE}&ca-cert-fingerprint={CA_FINGERPRINT}
```

Where `{CA_CERTIFICATE}` and `{CA_FINGERPRINT}` are valid `gocloud.dev/runtimevar` URI strings that will dereference to a valid Elasticsearch 8.x CA certificate and fingerprint value respectively. You can also pass in an optional `?es-password-uri=` parameter containing its own `gocloud.dev/runtimevar` URI string that will be dereferenced to an Elasticsearch password value if you don't want to include in the "{USER}:{PASSWORD}" constuct.

#### Related

* https://www.elastic.co/guide/en/elasticsearch/reference/8.7/configuring-stack-security.html
* https://gocloud.dev/runtimevar
* https://github.com/sfomuseum/runtimevar

### Elasticsearch 7.x

```
elasticsearch7://{ES_ENDPOINT}/{ES_INDEX}
```

### Elasticsearch 2.x

```
elasticsearch2://{ES_ENDPOINT}/{ES_INDEX}
```

## See also

* https://github.com/elastic/go-elasticsearch
* https://github.com/whosonfirst/go-writer
* https://github.com/whosonfirst/go-whosonfirst-iterate
* https://github.com/whosonfirst/go-whosonfirst-iterate-git
* https://github.com/whosonfirst/go-whosonfirst-iterwriter