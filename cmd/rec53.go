package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"rec53/server"
	"syscall"
)

func main() {
	flag.Parse()
	rec53 := server.NewServer("127.0.0.1:5353")
	rec53.Run()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping\n", s)
}
