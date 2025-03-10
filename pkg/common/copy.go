package common

import (
	"fmt"
	"reflect"
)

// CopyFields copies fields from src to dest.
// The dest and src can be different types but the field with same name
// must has the same type.
func CopyFields(dest, src any, fields ...string) error {
	if len(fields) == 0 {
		// Enforce the caller to specify fields to be copied.
		return fmt.Errorf("no field to be copied")
	}

	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("dest must be a pointer")
	}
	destValue = destValue.Elem()
	if destValue.Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a pointer to a struct")
	}

	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() != reflect.Ptr {
		return fmt.Errorf("src must be a pointer")
	}
	srcValue = srcValue.Elem()
	if srcValue.Kind() != reflect.Struct {
		return fmt.Errorf("src must be a pointer to a struct")
	}

	for _, name := range fields {
		srcFieldValue := srcValue.FieldByName(name)
		if !srcFieldValue.IsValid() {
			return fmt.Errorf("field %s not found in src", name)
		}
		destFieldValue := destValue.FieldByName(name)
		if !destFieldValue.IsValid() {
			return fmt.Errorf("field %s not found in dest", name)
		}
		if destFieldValue.Type().Name() != srcFieldValue.Type().Name() {
			return fmt.Errorf("type mismatched of field %s", name)
		}
		destFieldValue.Set(srcFieldValue)
	}

	return nil
}

// UpdateStructByMap set map's values to struct's fields,
// it will ignore fields that don't exist in the struct.
// return:
// updateFields: the updated fields to strcut, the slice element format is snake
func UpdateStructByMap(target any, source map[string]any) ([]string, error) {
	if len(source) == 0 {
		return nil, nil
	}

	if reflect.TypeOf(target).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("target must be a pointer")
	}

	targetValue := reflect.ValueOf(target).Elem()
	if targetValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("target must be a pointer to a struct")
	}

	var updateFields []string
	for key, value := range source {
		fieldValue := targetValue.FieldByName(key)

		if fieldValue.IsValid() && fieldValue.CanSet() {
			if fieldValue.Type() == reflect.TypeOf(value) {
				fieldValue.Set(reflect.ValueOf(value))
				updateFields = append(updateFields, CamelToSnake(key))
			} else {
				return nil, fmt.Errorf("type mismatch for field '%s'", key)
			}
		}
	}

	return updateFields, nil
}
