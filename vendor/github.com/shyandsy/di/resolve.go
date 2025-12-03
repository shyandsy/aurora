package di

import (
	"reflect"

	"github.com/pkg/errors"
)

func (c *container) Resolve(object interface{}) error {
	objectTp := reflect.TypeOf(object)
	objectVal := reflect.ValueOf(object)

	if object == nil {
		return errors.New("object cannot be nil")
	}
	if objectTp.Kind() != reflect.Pointer || objectTp.Elem().Kind() != reflect.Struct {
		return errors.New("object must be a pointer of struct")
	}

	objectVal = objectVal.Elem()

	element := objectTp.Elem()
	for i := 0; i < element.NumField(); i++ {
		tpField := element.Field(i)
		if _, ok := tpField.Tag.Lookup("inject"); !ok {
			continue
		}

		if tpField.Type.Kind() != reflect.Interface &&
			(tpField.Type.Kind() != reflect.Pointer || tpField.Type.Elem().Kind() != reflect.Struct) {
			return errors.New("inject field only for interface or *struct")
		}

		field := objectVal.FieldByName(tpField.Name)
		if !field.IsValid() || !field.CanSet() {
			return errors.New("inject field must CanSet")
		}

		s, err := c.parseStruct(tpField.Type)
		if err != nil {
			return err
		}

		dep, ok := c.singletonStore.Load(s.FullType())
		if !ok {
			if tpField.Type.Kind() == reflect.Pointer && tpField.Type.Elem().Kind() == reflect.Struct {
				newInstance := reflect.New(tpField.Type.Elem()).Interface()
				dep = newInstance
			} else {
				return errors.New("dependency not found: " + s.FullType())
			}
		}

		depVal := reflect.ValueOf(dep)
		if depVal.Kind() == reflect.Func {
			objs, err := c.Invoke(dep)
			if err != nil {
				return err
			}
			field.Set(objs[0])
		} else if field.Type().Kind() == reflect.Pointer && field.Type().Elem().Kind() == reflect.Struct {
			depVal := reflect.ValueOf(dep)

			if field.IsNil() {
				newTemp := reflect.New(field.Type().Elem())
				field.Set(newTemp)
			}

			if depVal.Type().AssignableTo(field.Type()) {
				field.Set(depVal)
			} else {
				fieldElem := field.Elem()
				depElem := depVal.Elem()
				if fieldElem.Type().Kind() == reflect.Struct && depElem.Type().Kind() == reflect.Struct {
					for i := 0; i < fieldElem.NumField(); i++ {
						fieldField := fieldElem.Field(i)
						fieldType := fieldElem.Type().Field(i)
						if _, hasInject := fieldType.Tag.Lookup("inject"); hasInject {
							continue
						}
						if fieldField.CanSet() {
							depField := depElem.FieldByName(fieldType.Name)
							if depField.IsValid() && depField.Type().AssignableTo(fieldField.Type()) {
								fieldField.Set(depField)
							}
						}
					}
				}
			}

			if err = c.Resolve(field.Interface()); err != nil {
				return err
			}

		} else if field.Type().Kind() == reflect.Interface {
			field.Set(reflect.ValueOf(dep))
		}
	}

	return nil
}
