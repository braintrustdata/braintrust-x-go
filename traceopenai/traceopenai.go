package traceopenai

import (
	"bytes"
	"io"
	"net/http"
)

type NextMiddleware = func(req *http.Request) (*http.Response, error)

func Middleware(req *http.Request, next NextMiddleware) (*http.Response, error) {

	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	resp, err := next(req)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return resp, nil
}
