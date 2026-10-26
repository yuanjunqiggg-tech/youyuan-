package py_rpc

import "fmt"

// Describe the type of PyRpc/StartType packet
const (
	StartTypeRequest  = "GetStartType"
	StartTypeResponse = "SetStartType"
)

// 机器人进服后的挑战之一
type StartType struct {
	Content string
	Type    string
}

// Return the name of g
func (s *StartType) Name() string {
	return s.Type
}

// Convert g to go object which only contains go-built-in types
func (s *StartType) MakeGo() (res any) {
	return []any{s.Content}
}

// Sync data to g from obj
func (s *StartType) FromGo(obj any) error {
	object, success := obj.([]any)
	if !success {
		return fmt.Errorf("FromGo：无法将 obj 转换为 []interface{}；obj = %#v", obj)
	}
	if len(object) != 1 {
		return fmt.Errorf("FromGo：object 的长度不等于 1；object = %#v", object)
	}
	// convert and check data
	s.Content, success = object[0].(string)
	if !success {
		return fmt.Errorf("FromGo：无法将 object[0] 转换为字符串；object[0] = %#v", object[0])
	}
	// get and sync data
	return nil
	// return
}
