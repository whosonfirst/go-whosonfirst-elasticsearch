package writer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	es "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/estransport"
	"github.com/elastic/go-elasticsearch/v7/esutil"
	"github.com/whosonfirst/go-whosonfirst-elasticsearch/document"
	"github.com/whosonfirst/go-whosonfirst-feature/properties"
	wof_writer "github.com/whosonfirst/go-writer/v2"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	ctx := context.Background()
	wof_writer.RegisterWriter(ctx, "elasticsearch", NewElasticsearchV7Writer)
	wof_writer.RegisterWriter(ctx, "elasticsearch7", NewElasticsearchV7Writer)
}

type ElasticsearchV7Writer struct {
	wof_writer.Writer
	indexer         esutil.BulkIndexer
	index_alt_files bool
	prepare_funcs   []document.PrepareDocumentFunc
	logger          *log.Logger
}

func NewElasticsearchV7Writer(ctx context.Context, uri string) (wof_writer.Writer, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URI, %w", err)
	}

	var es_endpoint string

	port := u.Port()

	switch port {
	case "443":
		es_endpoint = fmt.Sprintf("https://%s", u.Host)
	default:
		es_endpoint = fmt.Sprintf("http://%s", u.Host)
	}

	es_index := strings.TrimLeft(u.Path, "/")

	q := u.Query()

	retry := backoff.NewExponentialBackOff()

	es_cfg := es.Config{
		Addresses: []string{es_endpoint},

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

			es_logger := &estransport.ColorLogger{
				Output:             os.Stdout,
				EnableRequestBody:  true,
				EnableResponseBody: true,
			}

			es_cfg.Logger = es_logger
		}
	}

	es_client, err := es.NewClient(es_cfg)

	if err != nil {
		return nil, fmt.Errorf("Failed to create ES client, %w", err)
	}

	workers := 10

	str_workers := q.Get("workers")

	if str_workers != "" {

		w, err := strconv.Atoi(str_workers)

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
	}

	bi, err := esutil.NewBulkIndexer(bi_cfg)

	logger := log.Default()

	wr := &ElasticsearchV7Writer{
		indexer: bi,
		logger:  logger,
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

func (wr *ElasticsearchV7Writer) Write(ctx context.Context, path string, r io.ReadSeeker) (int64, error) {

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

	bulk_item := esutil.BulkIndexerItem{
		Action:     "index",
		DocumentID: doc_id,
		Body:       bytes.NewReader(enc_f),

		OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
			// log.Printf("Indexed %s\n", path)
		},

		OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
			if err != nil {
				log.Printf("ERROR: Failed to index %s, %s", path, err)
			} else {
				log.Printf("ERROR: Failed to index %s, %s: %s", path, res.Error.Type, res.Error.Reason)
			}
		},
	}

	err = wr.indexer.Add(ctx, bulk_item)

	if err != nil {
		return 0, fmt.Errorf("Failed to add bulk item for %s, %w", path, err)
	}

	return 0, nil
}

func (wr *ElasticsearchV7Writer) WriterURI(ctx context.Context, uri string) string {
	return uri
}

func (wr *ElasticsearchV7Writer) Close(ctx context.Context) error {
	return wr.indexer.Close(ctx)
}

func (wr *ElasticsearchV7Writer) Flush(ctx context.Context) error {
	return nil
}

func (wr *ElasticsearchV7Writer) SetLogger(ctx context.Context, logger *log.Logger) error {
	wr.logger = logger
	return nil
}

/*

index/es7yyyindex/es7yfunc PrepareFuncsFromFlagSet(ctx context.Context, fs *flag.FlagSet) ([]document.PrepareDocumentFunc, error) {

	index_spelunker_v1, err := lookup.BoolVar(fs, FLAG_INDEX_SPELUNKER_V1)

	if err != nil {
		return nil, err
	}

	append_spelunker_v1, err := lookup.BoolVar(fs, FLAG_APPEND_SPELUNKER_V1)

	if err != nil {
		return nil, err
	}

	index_only_props, err := lookup.BoolVar(fs, FLAG_INDEX_PROPS)

	if err != nil {
		return nil, err
	}

	if index_spelunker_v1 {

		if index_only_props {
			msg := fmt.Sprintf("-%s can not be used when -%s is enabled", FLAG_INDEX_PROPS, FLAG_INDEX_SPELUNKER_V1)
			return nil, errors.New(msg)
		}

		if append_spelunker_v1 {
			msg := fmt.Sprintf("-%s can not be used when -%s is enabled", FLAG_APPEND_SPELUNKER_V1, FLAG_INDEX_SPELUNKER_V1)
			return nil, errors.New(msg)
		}
	}

	prepare_funcs := make([]document.PrepareDocumentFunc, 0)

	if index_spelunker_v1 {
		prepare_funcs = append(prepare_funcs, document.PrepareSpelunkerV1Document)
	}

	if index_only_props {
		prepare_funcs = append(prepare_funcs, document.ExtractProperties)
	}

	if append_spelunker_v1 {
		prepare_funcs = append(prepare_funcs, document.AppendSpelunkerV1Properties)
	}

	return prepare_funcs, nil
}

*/
