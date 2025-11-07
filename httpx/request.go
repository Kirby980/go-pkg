package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Request struct {
	req          *http.Request
	err          error
	client       *http.Client
	queryBuilder *strings.Builder
}

func NewRequest(ctx context.Context, method, url string) *Request {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	return &Request{
		req:    req,
		err:    err,
		client: http.DefaultClient,
	}
}

func (r *Request) Client(client *http.Client) *Request {
	r.client = client
	return r
}

func (r *Request) JSONBody(val any) *Request {
	if r.err != nil {
		return r
	}
	body, err := json.Marshal(val)
	if err != nil {
		r.err = err
		return r
	}
	r.req.Body = io.NopCloser(bytes.NewBuffer(body))
	r.req.Header.Set("Content-Type", "application/json")
	return r
}

func (r *Request) AddHeader(key, value string) *Request {
	if r.err != nil {
		return r
	}
	r.req.Header.Set(key, value)
	return r
}
func (r *Request) Do() *Response {
	if r.err != nil {
		return &Response{
			err: r.err,
		}
	}
	r.applyParams()
	resp, err := r.client.Do(r.req)
	return &Response{
		Response: resp,
		err:      err,
	}
}

func (req *Request) AddParam(key, value string) *Request {
	if req.queryBuilder == nil {
		req.queryBuilder = &strings.Builder{}
	} else if req.queryBuilder.Len() > 0 {
		req.queryBuilder.WriteByte('&')
	}

	req.queryBuilder.WriteString(url.QueryEscape(key))
	req.queryBuilder.WriteByte('=')
	req.queryBuilder.WriteString(url.QueryEscape(value))

	return req
}

func (req *Request) applyParams() {
	if req.queryBuilder != nil {
		req.req.URL.RawQuery = req.queryBuilder.String()
	}
}
