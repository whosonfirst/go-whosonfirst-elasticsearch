package writer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/whosonfirst/go-whosonfirst-elasticsearch/document"
	"github.com/whosonfirst/go-whosonfirst-feature/properties"
	sp_document "github.com/whosonfirst/go-whosonfirst-spelunker/document"
	wof_writer "github.com/whosonfirst/go-writer/v3"
	es "gopkg.in/olivere/elastic.v3"
)

// https://github.com/olivere/elastic/wiki/BulkProcessor
// https://gist.github.com/olivere/a1dd52fc28cdfbbd6d4f

func init() {
	ctx := context.Background()
	wof_writer.RegisterWriter(ctx, "elasticsearch2", NewElasticsearchV2Writer)
}

type ElasticsearchV2Writer struct {
	wof_writer.Writer
	processor       *es.BulkProcessor
	index           string
	index_alt_files bool
	prepare_funcs   []document.PrepareDocumentFunc
	logger          *log.Logger
}

func NewElasticsearchV2Writer(ctx context.Context, uri string) (wof_writer.Writer, error) {

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

	workers := 10

	str_workers := q.Get("workers")

	if str_workers != "" {

		w, err := strconv.Atoi(str_workers)

		if err != nil {
			return nil, fmt.Errorf("Failed to parse ?workers= parameter, %w", err)
		}

		workers = w
	}

	es_client, err := es.NewClient(es.SetURL(es_endpoint))

	if err != nil {
		return nil, err
	}

	beforeCallback := func(executionId int64, req []es.BulkableRequest) {
		// log.Printf("Before commit %d, %d items\n", executionId, len(req))
	}

	afterCallback := func(executionId int64, requests []es.BulkableRequest, rsp *es.BulkResponse, err error) {

		if err != nil {
			log.Printf("Commit ID %d failed with error %v\n", executionId, err)
		}
	}

	// ugh, method chaining...

	bp, err := es_client.BulkProcessor().
		Name("Processor").
		FlushInterval(30 * time.Second).
		Workers(workers).
		BulkActions(1000).
		Stats(true).
		Before(beforeCallback).
		After(afterCallback).
		Do()

	if err != nil {
		return nil, fmt.Errorf("Failed to 'Do' processor, %w", err)
	}

	prepare_funcs := make([]document.PrepareDocumentFunc, 0)
	prepare_funcs = append(prepare_funcs, sp_document.PrepareSpelunkerV1Document)

	logger := log.Default()

	wr := &ElasticsearchV2Writer{
		processor:     bp,
		index:         es_index,
		prepare_funcs: prepare_funcs,
		logger:        logger,
	}

	return wr, nil
}

func (wr *ElasticsearchV2Writer) Write(ctx context.Context, path string, r io.ReadSeeker) (int64, error) {

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

	placetype, err := properties.Placetype(body)

	if err != nil {
		return 0, fmt.Errorf("Failed to derive placetype for %s, %w", path, err)
	}

	for _, f := range wr.prepare_funcs {

		new_body, err := f(ctx, body)

		if err != nil {
			return 0, fmt.Errorf("Failed to execute prepare func, %w", err)
		}

		body = new_body
	}

	var f interface{}
	err = json.Unmarshal(body, &f)

	if err != nil {
		return 0, fmt.Errorf("Failed to unmarshal %s, %v", path, err)
	}

	// Ugh... method chaning

	bulk_item := es.NewBulkIndexRequest().
		Id(doc_id).
		Index(wr.index).
		Type(placetype).
		Doc(f)

	wr.processor.Add(bulk_item)

	return 0, nil
}

func (wr *ElasticsearchV2Writer) WriterURI(ctx context.Context, uri string) string {
	return uri
}

func (wr *ElasticsearchV2Writer) Close(ctx context.Context) error {

	err := wr.Flush(ctx)

	if err != nil {
		return fmt.Errorf("Failed to flush writer, %w", err)
	}

	return wr.processor.Close()
}

func (wr *ElasticsearchV2Writer) Flush(ctx context.Context) error {

	err := wr.processor.Flush()

	if err != nil {
		return fmt.Errorf("Failed to flush processor, %w", err)
	}

	return nil
}

func (wr *ElasticsearchV2Writer) SetLogger(ctx context.Context, logger *log.Logger) error {
	wr.logger = logger
	return nil
}
