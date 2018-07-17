package martini

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/DataDog/dd-trace-go/tracer/tracertest"
	"github.com/go-martini/martini"
	"github.com/stretchr/testify/assert"
)

func TestTrace(t *testing.T) {
	assert := assert.New(t)
	testTracer, transport := tracertest.GetTestTracer()
	tracer.DefaultTracer = testTracer

	m := martini.Classic()
	m.Use(Middleware("foobar"))
	m.Get("/user/:id", func(params martini.Params, req *http.Request) string {
		_, ok := tracer.SpanFromContext(req.Context())
		assert.True(ok)
		return "test"
	})

	r := httptest.NewRequest("GET", "/user/123", nil)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	res := w.Result()
	assert.Equal(200, res.StatusCode)

	testTracer.ForceFlush()

	traces := transport.Traces()
	assert.Len(traces, 1)
	assert.Len(traces[0], 1)

	span := traces[0][0]
	assert.Equal(ext.HTTPType, span.Type)
	assert.Equal("foobar", span.Service)
	assert.Equal("/user/:id", span.Resource)
	assert.Equal(span.GetMeta(ext.HTTPMethod), "GET")
	assert.Equal(span.GetMeta(ext.HTTPURL), "/user/123")
	assert.Equal(span.GetMeta(ext.HTTPCode), "200")
}
