package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type objType string

const (
	objTypeVM  objType = "vm"
	objTypeVPN objType = "vpn"
)

type vmJSON struct {
	Setup    []string `json:"setup"`
	Teardown []string `json:"teardown"`
}

func (v *vmJSON) setup() *executor {
	return newExecutor(v.Setup)
}

func (v *vmJSON) teardown() *executor {
	return newExecutor(v.Teardown)
}

type vpnJSON struct {
	Setup    []string `json:"setup"`
	Teardown []string `json:"teardown"`
}

func (v *vpnJSON) setup() *executor {
	return newExecutor(v.Setup)
}

func (v *vpnJSON) teardown() *executor {
	return newExecutor(v.Teardown)
}

type ObjectJSON struct {
	Type      objType         `json:"type"`
	Name      string          `json:"name"`
	Awaits    []string        `json:"awaits"`
	Completes []string        `json:"completes"`
	Data      json.RawMessage `json:"data"`
	typeAttrs interface{}
	Children  []ObjectJSON `json:"children"`
}

func (o *ObjectJSON) makeTypeAttrs(t objType) (interface{}, error) {
	switch t {
	case objTypeVM:
		return &vmJSON{}, nil
	case objTypeVPN:
		return &vpnJSON{}, nil
	default:
		return nil, fmt.Errorf("unknown object type %s", t)
	}
}

func (o *ObjectJSON) resolveTypeAttrs() error {
	for i := range o.Children {
		if err := o.Children[i].resolveTypeAttrs(); err != nil {
			return err
		}
	}
	if o.Data == nil {
		return nil
	}
	var err error
	o.typeAttrs, err = o.makeTypeAttrs(o.Type)
	if err = json.Unmarshal(o.Data, o.typeAttrs); err != nil {
		return fmt.Errorf("cannot parse data attributes: %w", err)
	}
	return nil
}

func parseJSON(r io.Reader) (*ObjectJSON, error) {
	var o ObjectJSON
	d := json.NewDecoder(r)
	if err := d.Decode(&o); err != nil {
		return nil, fmt.Errorf("cannot decode to object: %w", err)
	}
	o.resolveTypeAttrs()
	return &o, nil
}

type nametype struct {
	name  string
	otype objType
}

type parser struct {
	futures        map[string]*future
	unresolvedFuts map[string]struct{}
	duplicatedFuts map[string]nametype
	objs           []*object
}

func newParser() *parser {
	return &parser{
		futures:        make(map[string]*future),
		unresolvedFuts: make(map[string]struct{}),
		duplicatedFuts: make(map[string]nametype),
		objs:           make([]*object, 0),
	}
}

func (p *parser) makeFuture(name string, otype objType) *future {
	f, exists := p.futures[name]
	if exists {
		if _, ok := p.unresolvedFuts[name]; ok {
			delete(p.unresolvedFuts, name)
		} else {
			p.duplicatedFuts[name] = nametype{name: name, otype: otype}
		}
		return f
	}
	f = newFuture(name)
	p.futures[name] = f
	return f
}

func (p *parser) makeFutureRef(name string) *future {
	f, ok := p.futures[name]
	if !ok {
		f = newFuture(name)
		p.futures[name] = f
		p.unresolvedFuts[name] = struct{}{}
	}
	return f
}

func (p *parser) hasUnresolvedFutures() bool {
	return len(p.unresolvedFuts) > 0
}

func (p *parser) hasDuplicatedFutures() bool {
	return len(p.duplicatedFuts) > 0
}

func (p *parser) convert(jo *ObjectJSON) {
	var lc lifecycle
	if jo.typeAttrs != nil {
		lc, _ = jo.typeAttrs.(lifecycle)
	}
	obj := &object{
		name:      jo.Name,
		lifecycle: lc,
	}
	for _, fname := range jo.Completes {
		f := p.makeFuture(fname, jo.Type)
		obj.completes = append(obj.completes, f)
	}
	for _, fname := range jo.Awaits {
		f := p.makeFutureRef(fname)
		obj.awaits = append(obj.awaits, f)
	}
	for _, ch := range jo.Children {
		p.convert(&ch)
	}
	p.objs = append(p.objs, obj)
}

func (p *parser) parse(r io.Reader) ([]*object, error) {
	objs, err := parseJSON(r)
	if err != nil {
		return nil, fmt.Errorf("cannot parse JSON: %w", err)
	}
	p.convert(objs)
	if p.hasUnresolvedFutures() {
		var names []string
		for name := range p.unresolvedFuts {
			names = append(names, name)
		}
		return nil, fmt.Errorf("futures %s are never completed", strings.Join(names, ", "))
	}
	if p.hasDuplicatedFutures() {
		var names []string
		for name := range p.duplicatedFuts {
			// TODO can print which object declares the future first
			names = append(names, name)
		}
		return nil, fmt.Errorf("futures %s are completed by multiple objects", strings.Join(names, ", "))
	}
	return p.objs, nil
}
