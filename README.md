# go-elasticsearch-whosonfirst

Go package for indexing Who's On First records in Elasticsearch.

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/whosonfirst/go-whosonfirst-elasticsearch.svg)](https://pkg.go.dev/github.com/whosonfirst/go-whosonfirst-elasticsearch)

Documentation is incomplete at this time.

## Tools

To build binary versions of these tools run the `cli` Makefile target. For example:

```
$> make cli
go build -mod vendor -o bin/es-whosonfirst-index cmd/es-whosonfirst-index/main.go
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

#### Elasticsearch "writer" URIs

This code assumes Elasticsearch 7.x by default although this may change with future releases. It is possible to specify the use of Elasticsearch 7.x or 2.x for indexing as follows:

### Default (Elasticsearch 7.x)

```
elasticsearch://{ES_ENDPOINT}/{ES_INDEX}
```

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