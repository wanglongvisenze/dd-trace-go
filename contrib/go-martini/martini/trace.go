package martini

import (
	"net/http"
	"reflect"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/go-martini/martini"
)

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

		rw := res.(martini.ResponseWriter)
		rw.Before(func(martini.ResponseWriter) {
			var r martini.Route
			r, ok := c.Get(reflect.TypeOf(&r).Elem()).Interface().(martini.Route)
			if ok {
				span.Resource = r.GetName()
				if span.Resource == "" {
					span.Resource = r.Pattern()
				}
			}
			span.Finish()
			// route := c.Get(reflect.TypeOf((martini.Route)(nil)))
			// if !route.IsNil() {
			// 	span.Resource = route.Interface().(martini.Route).GetName()
			// }
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
