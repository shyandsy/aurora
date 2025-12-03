package di

import (
	"reflect"

	"github.com/pkg/errors"
)

func (c *container) Invoke(f interface{}) ([]reflect.Value, error) {
	tp := reflect.TypeOf(f)
	val := reflect.ValueOf(f)

	if f == nil {
		return nil, errors.New("f cannot be nil")
	}
	if tp.Kind() != reflect.Func {
		return nil, errors.New("f must be a function")
	}

	var args []reflect.Value
	if tp.NumIn() > 0 {
		for i := 0; i < tp.NumIn(); i++ {
			inType := tp.In(i)

			if (inType.Kind() != reflect.Pointer || inType.Elem().Kind() != reflect.Struct) && inType.Kind() != reflect.Interface {
				return nil, errors.New("parameter must be interface or *struct")
			}

			s, err := c.parseStruct(inType)
			if err != nil {
				return nil, err
			}

			dep, ok := c.singletonStore.Load(s.FullType())
			if ok {
				arg := reflect.ValueOf(dep)

				if arg.Kind() == reflect.Func {
					objs, err := c.Invoke(dep)
					if err != nil {
						return nil, err
					}
					arg = objs[0]
				}

				if inType.Kind() == reflect.Pointer && inType.Elem().Kind() == reflect.Struct {
					if arg.Type().AssignableTo(inType) {
						hasInjectFields := false
						structType := inType.Elem()
						for i := 0; i < structType.NumField(); i++ {
							if _, ok := structType.Field(i).Tag.Lookup("inject"); ok {
								hasInjectFields = true
								break
							}
						}

						if hasInjectFields {
							ptrValue := reflect.New(inType.Elem()).Interface()
							if err = c.Resolve(ptrValue); err != nil {
								return nil, err
							}
							args = append(args, reflect.ValueOf(ptrValue))
						} else {
							args = append(args, arg)
						}
					} else {
						ptrValue := reflect.New(inType.Elem()).Interface()
						if err = c.Resolve(ptrValue); err != nil {
							return nil, err
						}
						args = append(args, reflect.ValueOf(ptrValue))
					}
					continue
				}

				if inType.Kind() == reflect.Interface {
					args = append(args, arg)
				} else if inType.Kind() == reflect.Struct {
					arg = arg.Elem()
					args = append(args, arg)
				} else {
					args = append(args, arg)
				}
				continue
			}
			return nil, errors.New("dependency not found: " + s.FullType())
		}
	}

	values := val.Call(args)

	return values, nil
}
