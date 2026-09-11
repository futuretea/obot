package proxy

import (
	"bytes"
	"net/http"
)

type callbackResponse struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func captureCallbackResponse(handler http.Handler, r *http.Request) *callbackResponse {
	resp := &callbackResponse{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
	handler.ServeHTTP(resp, r)
	return resp
}

func (r *callbackResponse) Header() http.Header {
	return r.header
}

func (r *callbackResponse) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *callbackResponse) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *callbackResponse) StatusCode() int {
	return r.statusCode
}

func (r *callbackResponse) WriteTo(w http.ResponseWriter) {
	for key, values := range r.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(r.statusCode)
	_, _ = w.Write(r.body.Bytes())
}
