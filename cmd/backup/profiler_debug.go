//go:build debug

package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
)

func init() {
	go func() {
		log.Println("Starting profiler on :6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
