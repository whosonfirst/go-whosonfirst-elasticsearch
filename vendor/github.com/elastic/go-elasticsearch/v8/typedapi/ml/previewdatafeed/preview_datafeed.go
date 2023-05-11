// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Code generated from the elasticsearch-specification DO NOT EDIT.
// https://github.com/elastic/elasticsearch-specification/tree/a4f7b5a7f95dad95712a6bbce449241cbb84698d

// Previews a datafeed.
package previewdatafeed

import (
	gobytes "bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const (
	datafeedidMask = iota + 1
)

// ErrBuildPath is returned in case of missing parameters within the build of the request.
var ErrBuildPath = errors.New("cannot build path, check for missing path parameters")

type PreviewDatafeed struct {
	transport elastictransport.Interface

	headers http.Header
	values  url.Values
	path    url.URL

	buf *gobytes.Buffer

	req *Request
	raw io.Reader

	paramSet int

	datafeedid string
}

// NewPreviewDatafeed type alias for index.
type NewPreviewDatafeed func() *PreviewDatafeed

// NewPreviewDatafeedFunc returns a new instance of PreviewDatafeed with the provided transport.
// Used in the index of the library this allows to retrieve every apis in once place.
func NewPreviewDatafeedFunc(tp elastictransport.Interface) NewPreviewDatafeed {
	return func() *PreviewDatafeed {
		n := New(tp)

		return n
	}
}

// Previews a datafeed.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/ml-preview-datafeed.html
func New(tp elastictransport.Interface) *PreviewDatafeed {
	r := &PreviewDatafeed{
		transport: tp,
		values:    make(url.Values),
		headers:   make(http.Header),
		buf:       gobytes.NewBuffer(nil),
	}

	return r
}

// Raw takes a json payload as input which is then passed to the http.Request
// If specified Raw takes precedence on Request method.
func (r *PreviewDatafeed) Raw(raw io.Reader) *PreviewDatafeed {
	r.raw = raw

	return r
}

// Request allows to set the request property with the appropriate payload.
func (r *PreviewDatafeed) Request(req *Request) *PreviewDatafeed {
	r.req = req

	return r
}

// HttpRequest returns the http.Request object built from the
// given parameters.
func (r *PreviewDatafeed) HttpRequest(ctx context.Context) (*http.Request, error) {
	var path strings.Builder
	var method string
	var req *http.Request

	var err error

	if r.raw != nil {
		r.buf.ReadFrom(r.raw)
	} else if r.req != nil {
		data, err := json.Marshal(r.req)

		if err != nil {
			return nil, fmt.Errorf("could not serialise request for PreviewDatafeed: %w", err)
		}

		r.buf.Write(data)
	}

	r.path.Scheme = "http"

	switch {
	case r.paramSet == datafeedidMask:
		path.WriteString("/")
		path.WriteString("_ml")
		path.WriteString("/")
		path.WriteString("datafeeds")
		path.WriteString("/")

		path.WriteString(r.datafeedid)
		path.WriteString("/")
		path.WriteString("_preview")

		method = http.MethodPost
	case r.paramSet == 0:
		path.WriteString("/")
		path.WriteString("_ml")
		path.WriteString("/")
		path.WriteString("datafeeds")
		path.WriteString("/")
		path.WriteString("_preview")

		method = http.MethodPost
	}

	r.path.Path = path.String()
	r.path.RawQuery = r.values.Encode()

	if r.path.Path == "" {
		return nil, ErrBuildPath
	}

	if ctx != nil {
		req, err = http.NewRequestWithContext(ctx, method, r.path.String(), r.buf)
	} else {
		req, err = http.NewRequest(method, r.path.String(), r.buf)
	}

	req.Header = r.headers.Clone()

	if req.Header.Get("Content-Type") == "" {
		if r.buf.Len() > 0 {
			req.Header.Set("Content-Type", "application/vnd.elasticsearch+json;compatible-with=8")
		}
	}

	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/vnd.elasticsearch+json;compatible-with=8")
	}

	if err != nil {
		return req, fmt.Errorf("could not build http.Request: %w", err)
	}

	return req, nil
}

// Perform runs the http.Request through the provided transport and returns an http.Response.
func (r PreviewDatafeed) Perform(ctx context.Context) (*http.Response, error) {
	req, err := r.HttpRequest(ctx)
	if err != nil {
		return nil, err
	}

	res, err := r.transport.Perform(req)
	if err != nil {
		return nil, fmt.Errorf("an error happened during the PreviewDatafeed query execution: %w", err)
	}

	return res, nil
}

// Do runs the request through the transport, handle the response and returns a previewdatafeed.Response
func (r PreviewDatafeed) Do(ctx context.Context) (Response, error) {

	response := NewResponse()

	res, err := r.Perform(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 299 {
		err = json.NewDecoder(res.Body).Decode(&response)
		if err != nil {
			return nil, err
		}

		return response, nil
	}

	errorResponse := types.NewElasticsearchError()
	err = json.NewDecoder(res.Body).Decode(errorResponse)
	if err != nil {
		return nil, err
	}

	return nil, errorResponse
}

// Header set a key, value pair in the PreviewDatafeed headers map.
func (r *PreviewDatafeed) Header(key, value string) *PreviewDatafeed {
	r.headers.Set(key, value)

	return r
}

// DatafeedId A numerical character string that uniquely identifies the datafeed. This
// identifier can contain lowercase
// alphanumeric characters (a-z and 0-9), hyphens, and underscores. It must
// start and end with alphanumeric
// characters. NOTE: If you use this path parameter, you cannot provide datafeed
// or anomaly detection job
// configuration details in the request body.
// API Name: datafeedid
func (r *PreviewDatafeed) DatafeedId(v string) *PreviewDatafeed {
	r.paramSet |= datafeedidMask
	r.datafeedid = v

	return r
}

// Start The start time from where the datafeed preview should begin
// API name: start
func (r *PreviewDatafeed) Start(v string) *PreviewDatafeed {
	r.values.Set("start", v)

	return r
}

// End The end time when the datafeed preview should stop
// API name: end
func (r *PreviewDatafeed) End(v string) *PreviewDatafeed {
	r.values.Set("end", v)

	return r
}
