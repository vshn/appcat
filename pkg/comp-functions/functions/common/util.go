package common

import "fmt"

// SetNestedObjectValue is necessary as unstructured can't handle anything except basic values and maps.
// this is a recursive function, it will traverse the map until it reaches the last element of the path.
// If it encounters any non-map values while traversing, it will throw an error.
func SetNestedObjectValue(values map[string]interface{}, path []string, val interface{}) error {

	if len(path) == 1 {
		source, isSlice := values[path[0]].([]interface{})
		if !isSlice {
			values[path[0]] = val
			return nil
		}
		toArrayValue, isSlice := val.([]interface{})
		if !isSlice {
			values[path[0]] = append(source, val)
			return nil
		}
		values[path[0]] = append(source, toArrayValue...)
		return nil
	}

	tmpVals, ok := values[path[0]].(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot traverse map, value at field %s is not a map", path[0])
	}

	return SetNestedObjectValue(tmpVals, path[1:], val)
}
