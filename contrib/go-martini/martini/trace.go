package martini

import (
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"sync"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/go-martini/martini"
)

type tracingResponseWriter struct {
	http.ResponseWriter
	context martini.Context

	// WriteHeader on the ResponseWriter may not be called, so we wrap both
	// methods and use a sync.Once to avoid filling the values multiple times
	sync.Once
	status   int
	resource string
}

func newTracingResponseWriter(w http.ResponseWriter, c martini.Context) *tracingResponseWriter {
	trw := &tracingResponseWriter{
		ResponseWriter: w,
		context:        c,
	}
	return trw
}

func (trw *tracingResponseWriter) Write(bs []byte) (int, error) {
	trw.fillFields(http.StatusOK)
	return trw.ResponseWriter.Write(bs)
}

func (trw *tracingResponseWriter) WriteHeader(statusCode int) {
	trw.fillFields(statusCode)
	trw.ResponseWriter.WriteHeader(statusCode)
}

func (trw *tracingResponseWriter) fillFields(statusCode int) {
	trw.Once.Do(func() {
		// set the status code
		trw.status = statusCode

		// We won't know the resource until after we've routed the request
		v := trw.context.Get(routeType)
		if !v.IsValid() {
			return
		}

		r, ok := v.Interface().(martini.Route)
		if !ok {
			return
		}

		// Use the name if it was provider, otherwise use the pattern
		trw.resource = firstNonEmpty(r.GetName(), r.Pattern())
	})
}

var routeType = reflect.TypeOf((*martini.Route)(nil)).Elem()

// Middleware creates a martini Handler which will trace incoming requests
func Middleware(service string) martini.Handler {
	t := tracer.DefaultTracer
	t.SetServiceInfo(service, "go-martini/martini", ext.AppTypeWeb)
	return func(w http.ResponseWriter, req *http.Request, c martini.Context) {
		if !t.Enabled() {
			c.Next()
			return
		}

		span, ctx := t.NewChildSpanWithContext("http.request", req.Context())
		defer span.Finish()

		trw := newTracingResponseWriter(w, c)

		// replace the response writer in the context
		c.MapTo(trw, (*http.ResponseWriter)(nil))

		span.Type = ext.HTTPType
		span.Service = service
		span.SetMeta(ext.HTTPMethod, req.Method)
		span.SetMeta(ext.HTTPURL, req.URL.Path)

		// Set the request context, then replace it in martini so that future
		// methods pick up the new context and are traced properly
		req = req.WithContext(ctx)
		c.Map(req)

		// Serve the next middleware
		c.Next()

		// set the resource
		span.Resource = firstNonEmpty(trw.resource, span.Resource)

		// set the status
		status := trw.status
		span.SetMeta(ext.HTTPCode, strconv.Itoa(status))
		if status >= 500 && status < 600 {
			span.FinishWithErr(errors.New(http.StatusText(status)))
		}
	}
}

func firstNonEmpty(strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return ""
}
