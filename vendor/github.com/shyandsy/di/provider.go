package di

import (
	"reflect"

	"github.com/pkg/errors"
)

func (c *container) Provide(object interface{}) error {
	tp := reflect.TypeOf(object)

	if object == nil {
		return errors.New("object cannot be nil")
	}

	if tp.Kind() == reflect.Pointer && tp.Elem().Kind() == reflect.Struct {
		return c.ProvideAs(object, object)
	} else if tp.Kind() == reflect.Func {
		if tp.NumOut() != 1 {
			return errors.New("the function for dependency creation should have exactly 1 return value")
		}
		out := tp.Out(0)

		if out.Kind() != reflect.Interface && (out.Kind() != reflect.Pointer || out.Elem().Kind() != reflect.Struct) {
			return errors.New("the function for dependency creation should return an interface or pointer of struct")
		}

		if out.Kind() != reflect.Interface {
			out = out.Elem()
		}

		return c.ProvideAs(object, reflect.New(out).Interface())
	}

	return errors.New("object must be pointer of struct")

}

func (c *container) ProvideAs(object interface{}, targetType interface{}) error {
	objectTp := reflect.TypeOf(object)
	targetTp := reflect.TypeOf(targetType)

	if object == nil {
		return errors.New("object cannot be nil")
	}
	if targetTp == nil {
		return errors.New("target cannot be nil")
	}
	// Validate object's kind BEFORE reflect.Value.IsNil(): IsNil panics on kinds that can't
	// be nil (struct, int, ...). If a caller passes a struct VALUE (e.g. a provider helper
	// returning `foo{}` instead of `&foo{}`), the old order hit IsNil first and crashed with
	// "reflect: call of reflect.Value.IsNil on struct Value" — a hard panic at registration,
	// with no compile error. Checking kind first turns that into this clean, returnable error.
	// Thanks to `||` short-circuit, objectTp.Elem() is never evaluated for a non-pointer, so
	// this check itself never panics.
	if (objectTp.Kind() != reflect.Pointer || objectTp.Elem().Kind() != reflect.Struct) && objectTp.Kind() != reflect.Func {
		return errors.New("object must be pointer of struct or function")
	}
	// Object is now guaranteed to be a Pointer or a Func — both are nilable, so IsNil is safe
	// and still catches a typed-nil pointer/func (e.g. (*Cat)(nil)).
	if reflect.ValueOf(object).IsNil() {
		return errors.New("object value cannot be nil")
	}
	if targetTp.Kind() != reflect.Pointer || (targetTp.Elem().Kind() != reflect.Interface && targetTp.Elem().Kind() != reflect.Struct) {
		return errors.New("target must be pointer of interface")
	}

	target := targetTp.Elem()

	if targetTp.Elem().Kind() == reflect.Interface {
		if objectTp.Kind() != reflect.Func && !objectTp.Implements(target) {
			return errors.New("object must implement target interface")
		}
	}

	s, err := c.parseStruct(targetTp)
	if err != nil {
		return err
	}

	fullTypeName := s.FullType()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.singletonStore.Store(fullTypeName, object)

	return nil
}
