package py_rpc

import "fmt"

// 机器人进服后的挑战之一
type SetMCPCheckNum []any

// Return the name of s
func (s *SetMCPCheckNum) Name() string {
	return "SetMCPCheckNum"
}

// Convert s to go object which only contains go-built-in types
func (s *SetMCPCheckNum) MakeGo() (res any) {
	return []any(*s)
}

// Sync data to s from obj
func (s *SetMCPCheckNum) FromGo(obj any) error {
	object, success := obj.([]any)
	if !success {
		return fmt.Errorf("FromGo：无法将 obj 转换为 []interface{}；obj = %#v", obj)
	}
	if len(object) != 1 {
		return fmt.Errorf("FromGo：object 的长度不等于 1；object = %#v", object)
	}
	// convert and check data
	*s, success = object[0].([]any)
	if !success {
		return fmt.Errorf(`FromGo: Failed to convert object[0] to []interface{}; object[0] = %#v`, object[0])
	}
	// get and sync data
	return nil
	// return
}
