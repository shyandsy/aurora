package di

import (
	"reflect"

	"github.com/pkg/errors"
)

type Struct struct {
	typePkg  string
	typeName string
	fullType string
	fields   []Field
}

type Field struct {
	name     string
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
		fields:   []Field{},
	}
	s.fullType = s.typePkg + ":" + s.typeName

	if tp.Kind() == reflect.Struct {
		if err := c.parseFields(s, tp); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (c *container) parseFields(s *Struct, tp reflect.Type) error {
	for i := 0; i < tp.NumField(); i++ {
		field := tp.Field(i)

		if tp.Kind() != reflect.Interface && (tp.Kind() != reflect.Pointer || tp.Elem().Kind() != reflect.Struct) {
			continue
		}

		if _, ok := field.Tag.Lookup("inject"); !ok {
			continue
		}

		f := Field{
			name:     field.Name,
			typePkg:  field.Type.PkgPath(),
			typeName: field.Type.Name(),
			fullType: field.Type.PkgPath() + ":" + field.Type.Name(),
		}
		s.fields = append(s.fields, f)
	}

	return nil
}
