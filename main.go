package main

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	elog *log.Logger
	dlog *log.Logger
	ilog *log.Logger
)

func initLogging(debug bool) {
	elog = log.New(os.Stderr, "error - ", log.LstdFlags)
	ilog = log.New(os.Stdout, "info - ", log.LstdFlags)
	dlog = log.New(ioutil.Discard, "", 0)
	if debug {
		dlog = log.New(os.Stdout, "debug - ", log.LstdFlags)
	}
}

func main() {
	initLogging(true)
	parser := newParser()
	objs, err := parser.parse(os.Stdin)
	if err != nil {
		elog.Printf("error: cannot accept scenario: %v", err)
		return
	}
	s := &scheduler{}
	s.run(objs)
}
