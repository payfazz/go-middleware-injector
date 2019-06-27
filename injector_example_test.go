package injector_test

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/payfazz/go-middleware"
	injector "github.com/payfazz/go-middleware-injector"
	"github.com/payfazz/go-middleware/common/kv"
	"github.com/payfazz/go-router/path"
)

func Example() {
	type method string

	h := middleware.Compile(
		kv.New(),
		injector.Use(func(r *http.Request) (method, *url.URL) {
			return method(r.Method), r.URL
		}),
		injector.Use(func(m method, u *url.URL, after injector.AfterNext) {
			start := time.Now()
			after(func() {
				elapsed := time.Since(start).Truncate(time.Millisecond)
				fmt.Printf("%s %s in %v\n", string(m), u.String(), elapsed)
			})
		}),
		path.H{
			"/hello": injector.Handler(func(w http.ResponseWriter) {
				fmt.Fprintln(w, "hello")
			}),
			"/sleep": injector.Handler(func(w http.ResponseWriter) {
				time.Sleep(2 * time.Second)
				fmt.Fprintln(w, "sleep")
			}),
		}.C(),
	)

	panic(http.ListenAndServe(":8080", h))
}
