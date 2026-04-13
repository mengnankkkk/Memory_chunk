package core

import "fmt"

type Registry struct {
	items map[string]Processor
}

func NewRegistry() *Registry {
	return &Registry{items: make(map[string]Processor)}
}

func (r *Registry) MustRegister(processor Processor) {
	if processor == nil {
		panic("nil processor")
	}
	name := processor.Descriptor().Name
	if _, exists := r.items[name]; exists {
		panic(fmt.Sprintf("processor already registered: %s", name))
	}
	r.items[name] = processor
}

func (r *Registry) Resolve(names ...string) []Processor {
	out := make([]Processor, 0, len(names))
	for _, name := range names {
		if processor, ok := r.items[name]; ok {
			out = append(out, processor)
		}
	}
	return out
}
