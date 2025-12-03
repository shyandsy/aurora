package di

import (
	"reflect"

	"github.com/pkg/errors"
)

func (c *container) Find(object interface{}) error {
	objectTp := reflect.TypeOf(object)
	objectVal := reflect.ValueOf(object)

	if object == nil {
		return errors.New("object cannot be nil")
	}
	if objectTp.Kind() != reflect.Pointer {
		return errors.New("object must be a pointer of struct or a pointer of interface")
	}

	elemKind := objectTp.Elem().Kind()

	var targetType reflect.Type
	if elemKind == reflect.Pointer {
		targetType = objectTp.Elem()
	} else if elemKind == reflect.Struct || elemKind == reflect.Interface {
		targetType = objectTp
	} else {
		return errors.New("object must be a pointer of struct, a pointer of interface, or a pointer to pointer of struct")
	}

	s, err := c.parseStruct(targetType)
	if err != nil {
		return err
	}

	target, ok := c.singletonStore.Load(s.FullType())
	if !ok {
		return errors.New("dependency not found: " + s.FullType())
	}

	targetVal := reflect.ValueOf(target)

	if !objectVal.Elem().CanSet() {
		return errors.New("object cannot be set")
	}

	if targetVal.Kind() == reflect.Func {
		objs, err := c.Invoke(target)
		if err != nil {
			return err
		}
		targetVal = objs[0]
	}

	if elemKind == reflect.Pointer {
		objectVal.Elem().Set(targetVal)
	} else if elemKind == reflect.Struct {
		objectVal.Elem().Set(targetVal.Elem())
		if err = c.Resolve(objectVal.Interface()); err != nil {
			return err
		}
	} else {
		objectVal.Elem().Set(targetVal)
	}

	return nil
}
