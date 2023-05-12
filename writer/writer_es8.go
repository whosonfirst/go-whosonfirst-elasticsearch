package writer

/*

./bin/wof-index-elasticsearch -monitor-uri counter://PT10S -writer-uri 'constant://?val=elasticsearch8%3A%2F%2Felastic%3A...%40localhost%3A9200%2Fcollection%3Fca-cert-uri%3Dfile%3A%2F%2F%2Fusr%2Flocal%2Fwhosonfirst%2Fgo-whosonfirst-elasticsearch%2Fhttp_ca.crt%26ca-fingerprint-uri%3Dfile%3A%2F%2F%2Fusr%2Flocal%2Fwhosonfirst%2Fgo-whosonfirst-elasticsearch%2Ffingerprint%26debug%3Dtrue%26bulk-index%3Dtrue' /usr/local/data/sfomuseum-data-architecture

*/

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/sfomuseum/runtimevar"
	"github.com/whosonfirst/go-whosonfirst-elasticsearch/document"
	"github.com/whosonfirst/go-whosonfirst-feature/properties"
	wof_writer "github.com/whosonfirst/go-writer/v3"
)

func init() {
	ctx := context.Background()
	wof_writer.RegisterWriter(ctx, "elasticsearch8", NewElasticsearchV8Writer)
}

// ElasticsearchV8Writer is a struct that implements the `Writer` interface for writing documents to an Elasticsearch
// index using the github.com/elastic/go-elasticsearch/v8 package.
type ElasticsearchV8Writer struct {
	wof_writer.Writer
	client          *es.Client
	index           string
	indexer         esutil.BulkIndexer
	index_alt_files bool
	prepare_funcs   []document.PrepareDocumentFunc
	logger          *log.Logger
	waitGroup       *sync.WaitGroup
}

// NewElasticsearchV8Writer returns a new `ElasticsearchV8Writer` instance for writing documents to an
// Elasticsearch index using the github.com/elastic/go-elasticsearch/v8 package configured by 'uri' which
// is expected to take the form of:
//
//	elasticsearch://{USER}:{PASSWORD}@{HOST}:{PORT}/{INDEX}?{QUERY_PARAMETERS}
//	elasticsearch7://{USER}:{PASSWORD}@{HOST}:{PORT}/{INDEX}?{QUERY_PARAMETERS}
//
// Where {QUERY_PARAMETERS} may be one or more of the following:
// * ?ca-cert-uri={STRING}. A valid gocloud.dev/runtimevar URI which, when dereferenced, will contain the Elasticsearch CA certificate to use for HTTPS requests.
// * ?es-password-uri={STRING}. An optional gocloud.dev/runtimevar URI which, when dereferenced, will contain the password for the {USER} account when making requests. If present, this value supersedes any password set in the "{USER}:{PATH}" component of 'uri'.
// * ?debug={BOOLEAN}. If true then verbose Elasticsearch logging for requests and responses will be enabled. Default is false.
// * ?bulk-index={BOOLEAN}. If true then writes will be performed using a "bulk indexer". Default is true.
// * ?workers={INT}. The number of users to enable for bulk indexing. Default is 10.
func NewElasticsearchV8Writer(ctx context.Context, uri string) (wof_writer.Writer, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URI, %w", err)
	}

	es_endpoint := fmt.Sprintf("https://%s", u.Host)

	es_index := strings.TrimLeft(u.Path, "/")

	es_user := u.User.Username()
	es_pswd, _ := u.User.Password()

	q := u.Query()

	q_cert_uri := q.Get("ca-cert-uri")
	q_fingerprint_uri := q.Get("ca-fingerprint-uri")
	q_pswd_uri := q.Get("es-password-uri")

	runtime_ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// ! ERROR: tls: failed to verify certificate: x509: certificate signed by unknown authority (possibly because of "crypto/rsa: verification error" while trying to verify candidate authority certificate "Elasticsearch security auto-configuration HTTP CA")

	es_cert, err := runtimevar.StringVar(runtime_ctx, q_cert_uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to load CA cert runtimevar URI, %w", err)
	}

	es_cert_fingerprint, err := runtimevar.StringVar(runtime_ctx, q_fingerprint_uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to load CA fingerprint runtimevar URI, %w", err)
	}

	if q_pswd_uri != "" {

		pswd, err := runtimevar.StringVar(runtime_ctx, q_pswd_uri)

		if err != nil {
			return nil, fmt.Errorf("Failed to load ES password URI, %w", err)
		}

		es_pswd = pswd
	}

	retry := backoff.NewExponentialBackOff()

	es_cfg := es.Config{
		Addresses: []string{es_endpoint},

		Username: es_user,
		Password: es_pswd,

		// The documentation for custom CAs in ES8 is kind of all over the map with the main docs
		// not jibing with the "security" docs. Likewise, without passing in the CertificateFingerprint
		// everything fails with unknown CA errors. On the other hand, even with the argument everything
		// still fails but with different and unspecified errors:
		// "2023/05/11 10:13:46 Failed to iterate, Failed to iterate with writer, Failed to close ES writer, One or more Close operations failed: Indexed (522) documents with (1065) errors exit status 1"
		//
		// https://github.com/elastic/go-elasticsearch
		// https://github.com/elastic/go-elasticsearch/blob/main/_examples/security/tls_configure_ca.go

		CACert:                 []byte(es_cert),
		CertificateFingerprint: es_cert_fingerprint,

		RetryOnStatus: []int{502, 503, 504, 429},
		RetryBackoff: func(i int) time.Duration {
			if i == 1 {
				retry.Reset()
			}
			return retry.NextBackOff()
		},
		MaxRetries: 5,
	}

	str_debug := q.Get("debug")

	if str_debug != "" {

		debug, err := strconv.ParseBool(str_debug)

		if err != nil {
			return nil, fmt.Errorf("Failed to parse ?debug= parameter, %w", err)
		}

		if debug {

			// https://github.com/elastic/go-elasticsearch/blob/67ab061a41de345b2af16ba4b67ccf6fd164f497/_examples/logging/default.go

			es_logger := &elastictransport.TextLogger{
				Output: os.Stdout,
			}

			es_cfg.Logger = es_logger
		}
	}

	es_client, err := es.NewClient(es_cfg)

	if err != nil {
		return nil, fmt.Errorf("Failed to create ES client, %w", err)
	}

	logger := log.New(io.Discard, "", 0)

	wg := new(sync.WaitGroup)

	wr := &ElasticsearchV8Writer{
		client:    es_client,
		index:     es_index,
		logger:    logger,
		waitGroup: wg,
	}

	bulk_index := true

	q_bulk_index := q.Get("bulk-index")

	if q_bulk_index != "" {

		v, err := strconv.ParseBool(q_bulk_index)

		if err != nil {
			return nil, fmt.Errorf("Failed to parse ?bulk-index= parameter, %w", err)
		}

		bulk_index = v
	}

	if bulk_index {

		workers := 10

		q_workers := q.Get("workers")

		if q_workers != "" {

			w, err := strconv.Atoi(q_workers)

			if err != nil {
				return nil, fmt.Errorf("Failed to parse ?workers= parameter, %w", err)
			}

			workers = w
		}

		bi_cfg := esutil.BulkIndexerConfig{
			Index:         es_index,
			Client:        es_client,
			NumWorkers:    workers,
			FlushInterval: 30 * time.Second,
			OnError: func(ctx context.Context, err error) {
				wr.logger.Printf("ES bulk indexer reported an error: %v\n", err)
			},
			// OnFlushStart func(context.Context) context.Context // Called when the flush starts.
			OnFlushEnd: func(ctx context.Context) {
				wr.logger.Printf("ES bulk indexer flush end")
			},
		}

		bi, err := esutil.NewBulkIndexer(bi_cfg)

		if err != nil {
			return nil, fmt.Errorf("Failed to create bulk indexer, %w", err)
		}

		wr.indexer = bi
	}

	str_index_alt := q.Get("index-alt-files")

	if str_index_alt != "" {
		index_alt_files, err := strconv.ParseBool(str_index_alt)

		if err != nil {
			return nil, fmt.Errorf("Failed to parse ?index-alt-files parameter, %w", err)
		}

		wr.index_alt_files = index_alt_files
	}

	prepare_funcs := make([]document.PrepareDocumentFunc, 0)

	prepare_funcs = append(prepare_funcs, document.PrepareSpelunkerV1Document)

	wr.prepare_funcs = prepare_funcs

	return wr, nil
}

// Write copies the content of 'fh' to the Elasticsearch index defined in `NewElasticsearchV8Writer`.
func (wr *ElasticsearchV8Writer) Write(ctx context.Context, path string, r io.ReadSeeker) (int64, error) {

	body, err := io.ReadAll(r)

	if err != nil {
		return 0, fmt.Errorf("Failed to read body for %s, %w", path, err)
	}

	id, err := properties.Id(body)

	if err != nil {
		return 0, fmt.Errorf("Failed to derive ID for %s, %w", path, err)
	}

	doc_id := strconv.FormatInt(id, 10)

	alt_label, err := properties.AltLabel(body)

	if err != nil {
		return 0, fmt.Errorf("Failed to derive alt label for %s, %w", path, err)
	}

	if alt_label != "" {

		if !wr.index_alt_files {
			return 0, nil
		}

		doc_id = fmt.Sprintf("%s-%s", doc_id, alt_label)
	}

	// START OF manipulate body here...

	for _, f := range wr.prepare_funcs {

		new_body, err := f(ctx, body)

		if err != nil {
			return 0, fmt.Errorf("Failed to execute prepare func, %w", err)
		}

		body = new_body
	}

	// END OF manipulate body here...

	var f interface{}
	err = json.Unmarshal(body, &f)

	if err != nil {
		return 0, fmt.Errorf("Failed to unmarshal %s, %v", path, err)
	}

	enc_f, err := json.Marshal(f)

	if err != nil {
		return 0, fmt.Errorf("Failed to marshal %s, %v", path, err)
	}

	// Do NOT bulk index. For example if you are using this in concert with
	// go-writer.MultiWriter running in async mode in a Lambda function where
	// the likelihood of that code being re-used across invocations is high.
	// The problem is that the first invocation will call wr.indexer.Close()
	// but then the second invocation, using the same code, will call wr.indexer.Add()
	// which will trigger a panic because the code (in esutil) will try to send
	// data on a closed channel. Computers...

	if wr.indexer == nil {

		wr.waitGroup.Add(1)
		defer wr.waitGroup.Done()

		req := esapi.IndexRequest{
			Index:      wr.index,
			DocumentID: doc_id,
			Body:       bytes.NewReader(enc_f),
			Refresh:    "true",
		}

		rsp, err := req.Do(ctx, wr.client)

		if err != nil {
			return 0, fmt.Errorf("Error getting response: %w", err)
		}

		defer rsp.Body.Close()

		if rsp.IsError() {

			var error_msg string

			var buf bytes.Buffer
			buf_wr := bufio.NewWriter(&buf)

			_, err := io.Copy(buf_wr, rsp.Body)

			if err == nil {
				buf_wr.Flush()
				error_msg = buf.String()
			}

			return 0, fmt.Errorf("Failed to index document, %s: %s", rsp.Status(), error_msg)
		}

		return 0, nil
	}

	// Do bulk index

	wr.waitGroup.Add(1)

	bulk_item := esutil.BulkIndexerItem{
		Action:     "index",
		DocumentID: doc_id,
		Body:       bytes.NewReader(enc_f),

		OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
			wr.logger.Printf("Indexed %s as %s\n", path, doc_id)
			wr.waitGroup.Done()
		},

		OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
			if err != nil {
				wr.logger.Printf("ERROR: Failed to index %s, %s", path, err)
			} else {
				wr.logger.Printf("ERROR: Failed to index %s, %s: %s", path, res.Error.Type, res.Error.Reason)
			}

			wr.waitGroup.Done()
		},
	}

	err = wr.indexer.Add(ctx, bulk_item)

	if err != nil {
		return 0, fmt.Errorf("Failed to add bulk item for %s, %w", path, err)
	}

	return 0, nil
}

// WriterURI returns 'uri' unchanged
func (wr *ElasticsearchV8Writer) WriterURI(ctx context.Context, uri string) string {
	return uri
}

// Close waits for all pending writes to complete and closes the underlying writer mechanism.
func (wr *ElasticsearchV8Writer) Close(ctx context.Context) error {

	// Do NOT bulk index

	if wr.indexer == nil {
		wr.waitGroup.Wait()
		return nil
	}

	// Do bulk index

	err := wr.indexer.Close(ctx)

	if err != nil {
		return fmt.Errorf("Failed to close indexer, %w", err)
	}

	wr.waitGroup.Wait()

	stats := wr.indexer.Stats()

	if stats.NumFailed > 0 {
		return fmt.Errorf("Indexed (%d) documents with (%d) errors", stats.NumFlushed, stats.NumFailed)
	}

	wr.logger.Printf("Successfully indexed (%d) documents", stats.NumFlushed)
	return nil
}

// Flush() does nothing in a `ElasticsearchV8Writer` context.
func (wr *ElasticsearchV8Writer) Flush(ctx context.Context) error {
	return nil
}

// SetLogger assigns 'logger' to 'wr'.
func (wr *ElasticsearchV8Writer) SetLogger(ctx context.Context, logger *log.Logger) error {
	wr.logger = logger
	return nil
}
