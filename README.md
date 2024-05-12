telemetry
=========

Go package to simplify recording metrics in a Prometheus compatible format.
Its based on github.com/uber-go/tally/v4/prometheus.

Checkout the [example](_examples/basic/main.go) for more details.

Compatible with chi v4.

## Usage
```go
r := chi.NewRouter()
r.Use(telemetry.Collector(telemetry.Config{
	AllowAny: true,
	AsteriskAltenative: "XXX",
}, []string{"/api"})) // path prefix filters records generic http request metrics
```

## Example
```go
r.Route("/api/users", func(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Get Users"))
	})
	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Get User"))
	})
	r.Get("/{id}/posts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Get User Posts"))
	})
	r.Get("/{id}/posts/*", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Get User Post"))
	})
})
```

### With chi route with asterisk ( `*` )
if you access to the route, such as `/api/users/1111/posts/2222`, metric endpoint label is the following.
```
http_request_duration_seconds_count{endpoint="get__api_users__id__posts_XXX",status="200"} 2
```
