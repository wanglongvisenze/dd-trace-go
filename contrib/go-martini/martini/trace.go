package martini

import (
	"net/http"
	"reflect"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/go-martini/martini"
)

var routeType = reflect.TypeOf((*martini.Route)(nil)).Elem()

// Middleware creates a martini Handler which will trace incoming requests
func Middleware(service string) martini.Handler {
	t := tracer.DefaultTracer
	t.SetServiceInfo(service, "go-martini/martini", ext.AppTypeWeb)
	return func(res http.ResponseWriter, req *http.Request, c martini.Context) {
		if !t.Enabled() {
			c.Next()
			return
		}

		span, ctx := t.NewChildSpanWithContext("http.request", req.Context())
		defer span.Finish()

		// We won't know the resource until after we've routed the request
		rw := res.(martini.ResponseWriter)
		rw.Before(func(martini.ResponseWriter) {
			v := c.Get(routeType)
			if !v.IsValid() {
				return
			}

			r, ok := v.Interface().(martini.Route)
			if !ok {
				return
			}

			// Use the name if it was provider, otherwise use the pattern
			span.Resource = firstNonEmpty(r.GetName(), r.Pattern(), span.Resource)
		})

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
