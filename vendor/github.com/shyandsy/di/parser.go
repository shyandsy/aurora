package di

import (
	"reflect"

	"github.com/pkg/errors"
)

type Struct struct {
	typePkg  string
	typeName string
	fullType string
}

func (s Struct) TypePkg() string {
	return s.typePkg
}

func (s Struct) TypeName() string {
	return s.typeName
}

func (s Struct) FullType() string {
	return s.fullType
}

func (c *container) parseStruct(tp reflect.Type) (*Struct, error) {
	if tp == nil {
		return nil, errors.New("tp must be not nil")
	}

	if (tp.Kind() != reflect.Pointer || tp.Elem().Kind() != reflect.Struct) &&
		(tp.Kind() != reflect.Pointer || tp.Elem().Kind() != reflect.Interface) &&
		tp.Kind() != reflect.Struct &&
		tp.Kind() != reflect.Interface {
		return nil, errors.New("tp must be a struct | interface, or a pointer of struct | interface")
	}

	if tp.Kind() == reflect.Pointer {
		tp = tp.Elem()
	}

	s := &Struct{
		typePkg:  tp.PkgPath(),
		typeName: tp.Name(),
	}
	s.fullType = s.typePkg + ":" + s.typeName

	return s, nil
}
