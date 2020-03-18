package main

import (
	"log"
	"sync"
)

type future struct {
	mux sync.Mutex

	name   string
	done   bool
	reason error
}

type fawait interface {
	result() (bool, error)
}

type fcomplete interface {
	complete()
	fail(err error)
}

func newFuture(name string) *future {
	return &future{name: name}
}

func (f *future) result() (bool, error) {
	f.mux.Lock()
	defer f.mux.Unlock()

	return f.done, f.reason
}

func (f *future) complete() {
	f.mux.Lock()
	defer f.mux.Unlock()

	f.done = true
}

func (f *future) fail(err error) {
	f.mux.Lock()
	defer f.mux.Unlock()

	f.done = true
	f.reason = err
}

type objectStatus int

const (
	objectStatusUnknown = iota
	objectStatusRunning
	objectStatusCreated
	objectStatusUnresolved
)

func (s objectStatus) consumed() bool {
	switch s {
	case objectStatusRunning, objectStatusCreated:
		return true
	default:
		return false
	}
}

type object struct {
	name      string
	status    objectStatus
	awaits    []fawait
	completes []fcomplete
}

func (o *object) String() string {
	return o.name
}

func (o *object) create() error {
	o.status = objectStatusRunning
	// TODO wait
	o.status = objectStatusCreated
	//return errors.New("failed")
	return nil
}

func (o *object) failedDeps() error {
	if o.status.consumed() {
		return nil
	}
	for i := range o.awaits {
		if done, err := o.awaits[i].result(); done && err != nil {
			o.status = objectStatusUnresolved
			return err
		}
	}
	return nil
}

func (o *object) ready() bool {
	if o.status.consumed() {
		return false
	}
	for i := range o.awaits {
		if done, _ := o.awaits[i].result(); !done {
			return false
		}
	}
	return true
}

type scheduler struct{}

func (s *scheduler) cascade(o *object, err error) {
	for i := range o.completes {
		o.completes[i].fail(err)
	}
}

func (s *scheduler) create(o *object) error {
	if err := o.create(); err != nil {
		for i := range o.completes {
			o.completes[i].fail(err)
		}
		return err
	}
	for i := range o.completes {
		o.completes[i].complete()
	}
	return nil
}

func (s *scheduler) run(objs []*object) {
	for {
		var worked bool
		for _, o := range objs {
			if err := o.failedDeps(); err != nil {
				s.cascade(o, err)
				log.Printf("not creating %s", o)
				worked = true
				continue
			}
			if o.ready() {
				if err := s.create(o); err != nil {
					log.Printf("creation of %s failed: %v", o, err)
				} else {
					log.Printf("created %s", o)
				}
				worked = true
			}
		}
		if !worked {
			log.Printf("nothing left to do")
			return
		}
	}
}
