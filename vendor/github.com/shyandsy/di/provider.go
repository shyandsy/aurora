package di

import (
	"reflect"

	"github.com/pkg/errors"
)

// Provide registers a singleton instance in the container.
//
// object must be a struct pointer (e.g. &Cat{}). The exact same instance is
// stored and handed back on every Find/Resolve/Invoke — i.e. it is a singleton.
//
// Functions are NOT accepted. Passing a func (e.g. Provide(func() *Cat {...}))
// returns an error instead of silently registering a per-resolution factory:
// a caller who expected a shared singleton would otherwise have silently gotten
// a fresh instance on every resolution. If a per-resolution (transient) factory
// is ever needed, that belongs in a separate, explicitly named API (e.g. a
// future ProvideFactory), not in Provide.
func (c *container) Provide(object interface{}) error {
	if object == nil {
		return errors.New("object cannot be nil")
	}

	tp := reflect.TypeOf(object)

	if tp.Kind() == reflect.Pointer && tp.Elem().Kind() == reflect.Struct {
		return c.ProvideAs(object, object)
	}

	if tp.Kind() == reflect.Func {
		return errors.New("di: Provide expects a struct pointer (a singleton instance); functions are not supported")
	}

	return errors.New("object must be pointer of struct")
}

// ProvideAs registers a singleton instance under a specific interface (or
// concrete) type, so callers can program to that interface.
//
// object must be a struct pointer that implements targetType; targetType is a
// pointer to the interface/struct type, e.g. (*Pet)(nil). The exact same
// instance is stored and handed back on every resolution — i.e. it is a
// singleton.
//
// Functions are NOT accepted (same reasoning as Provide): a func would silently
// become a per-resolution factory rather than a shared singleton. A transient
// factory, if ever needed, belongs in a separate, explicitly named API.
func (c *container) ProvideAs(object interface{}, targetType interface{}) error {
	objectTp := reflect.TypeOf(object)
	targetTp := reflect.TypeOf(targetType)

	if object == nil {
		return errors.New("object cannot be nil")
	}
	if targetTp == nil {
		return errors.New("target cannot be nil")
	}
	// Reject functions explicitly: a func would otherwise be stored and re-invoked
	// on every resolution (a silent per-resolution factory), which is not what a
	// singleton container promises.
	if objectTp.Kind() == reflect.Func {
		return errors.New("di: ProvideAs expects a struct pointer (a singleton instance); functions are not supported")
	}
	// Validate object's kind BEFORE reflect.Value.IsNil(): IsNil panics on kinds that can't
	// be nil (struct, int, ...). If a caller passes a struct VALUE (e.g. a provider helper
	// returning `foo{}` instead of `&foo{}`), the old order hit IsNil first and crashed with
	// "reflect: call of reflect.Value.IsNil on struct Value" — a hard panic at registration,
	// with no compile error. Checking kind first turns that into this clean, returnable error.
	// Thanks to `||` short-circuit, objectTp.Elem() is never evaluated for a non-pointer, so
	// this check itself never panics.
	if objectTp.Kind() != reflect.Pointer || objectTp.Elem().Kind() != reflect.Struct {
		return errors.New("object must be pointer of struct")
	}
	// Object is now guaranteed to be a struct Pointer — nilable, so IsNil is safe
	// and still catches a typed-nil pointer (e.g. (*Cat)(nil)).
	if reflect.ValueOf(object).IsNil() {
		return errors.New("object value cannot be nil")
	}
	if targetTp.Kind() != reflect.Pointer || (targetTp.Elem().Kind() != reflect.Interface && targetTp.Elem().Kind() != reflect.Struct) {
		return errors.New("target must be pointer of interface")
	}

	target := targetTp.Elem()

	if targetTp.Elem().Kind() == reflect.Interface {
		if !objectTp.Implements(target) {
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
