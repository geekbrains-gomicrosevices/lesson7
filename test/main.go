package main

import "net/http"

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		panic("Oops")
	})
	http.ListenAndServe(":9999", nil)
}
