package martini_test

import (
	martinitrace "github.com/DataDog/dd-trace-go/contrib/go-martini/martini"
	"github.com/go-martini/martini"
)

// To start tracing requests, add the trace middleware to your Martini
func Example() {
	m := martini.Classic()

	// Use the tracer middleware with your desired service name.
	m.Use(martinitrace.Middleware("my-web-app"))

	m.Router.Get("/hello", func() string {
		return "hello world!"
	})

	// And start gathering request traces.
	m.Run()
}
