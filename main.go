package main

import (
	"errors"
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

type object struct {
	name      string
	created   bool
	awaits    []fawait
	completes []fcomplete
}

func (o *object) String() string {
	return o.name
}

func (o *object) create() error {
	o.created = true
	return errors.New("failed")
}

func (o *object) failedDeps() error {
	if o.created {
		return nil
	}
	for i := range o.awaits {
		if done, err := o.awaits[i].result(); done && err != nil {
			o.created = true
			return err
		}
	}
	return nil
}

func (o *object) ready() bool {
	if o.created {
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

func (s *scheduler) run(objs []object) {
	for {
		var worked bool
		for i := range objs {
			o := &objs[i]
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

func main() {
	f1 := newFuture("f1")
	f2 := newFuture("f2")
	o1 := object{
		name:      "o1",
		awaits:    []fawait{f1},
		completes: []fcomplete{f2},
	}
	o2 := object{
		name:      "o2",
		completes: []fcomplete{f1},
	}
	s := &scheduler{}
	s.run([]object{o1, o2})
}
