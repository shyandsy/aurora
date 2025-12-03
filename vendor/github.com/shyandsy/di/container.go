package di

import (
	"reflect"
	"sync"
)

type Container interface {
	Provide(object interface{}) error
	ProvideAs(object interface{}, tp interface{}) error
	Find(object interface{}) error
	Resolve(object interface{}) error
	Invoke(f interface{}) ([]reflect.Value, error)
}

type container struct {
	mu             *sync.Mutex
	singletonStore sync.Map
	typeCache      sync.Map
}

func NewContainer() Container {
	return &container{
		mu:             &sync.Mutex{},
		singletonStore: sync.Map{},
	}
}
