package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipResponseWriter wraps http.ResponseWriter to gzip the response body.
// It transparently detects whether to compress based on the Content-Type
// header set by the wrapped handler. Binary content (images, fonts, video)
// is passed through uncompressed.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz            *gzip.Writer
	acceptsGzip   bool
	headerWritten bool
	compressing   bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.headerWritten {
		return
	}
	w.headerWritten = true

	if w.acceptsGzip {
		ct := w.Header().Get("Content-Type")
		if shouldGzip(ct) {
			w.compressing = true
			w.Header().Set("Content-Encoding", "gzip")
			// Content-Length is now invalid; remove it so the response is
			// chunked instead of truncating.
			w.Header().Del("Content-Length")
		}
	}

	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	if w.compressing {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if w.compressing {
		w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// shouldGzip returns true for content types that benefit from gzip.
// Binary assets (images, fonts, video) are already compressed and skipped.
func shouldGzip(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "javascript") ||
		strings.Contains(contentType, "xml")
}

// gzipPool reuses gzip writers to avoid allocating on every request.
var gzipPool = sync.Pool{
	New: func() interface{} {
		// Level 5 is a good trade-off between speed and compression for HTML/JSON.
		w, _ := gzip.NewWriterLevel(io.Discard, 5)
		return w
	},
}

// GzipMiddleware compresses responses with gzip when the client accepts it
// and the content type is compressible. SSE and binary responses are skipped.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if the client doesn't accept gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip SSE — streaming responses must not be buffered/compressed
		if strings.Contains(r.URL.Path, "/api/jobs/stream") {
			next.ServeHTTP(w, r)
			return
		}

		// Hint caches that the response varies based on Accept-Encoding
		w.Header().Set("Vary", "Accept-Encoding")

		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer gzipPool.Put(gz)

		gw := &gzipResponseWriter{ResponseWriter: w, gz: gz, acceptsGzip: true}
		next.ServeHTTP(gw, r)
		if gw.compressing {
			gz.Close()
		}
	})
}