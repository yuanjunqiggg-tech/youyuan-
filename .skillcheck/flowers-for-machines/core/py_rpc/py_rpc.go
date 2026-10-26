package py_rpc

import (
	"encoding/json"
	"fmt"
)

// Convert py_rpc to go object which
// only contains go-built-in types
func Marshal(py_rpc PyRpc) (res any) {
	return []any{
		py_rpc.Name(),
		py_rpc.MakeGo(),
		nil,
	}
}

// Convert obj to PyRpc
func Unmarshal(obj any) (py_rpc PyRpc, err error) {
	object, success := obj.([]any)
	if !success {
		return nil, fmt.Errorf("Unmarshal：不支持的 PyRpc 数据包；obj = %#v", obj)
	}
	if len(object) < 2 {
		return nil, fmt.Errorf("Unmarshal：object 的长度小于 2；object = %#v", object)
	}
	// check data
	name, success := object[0].(string)
	if !success {
		return nil, fmt.Errorf("Unmarshal：无法将 object[0] 转换为字符串；object[0] = %#v", object[0])
	}
	// get name
	var ok bool
	py_rpc, ok = Pool()[name]
	if !ok {
		py_rpc = &Default{NAME: name}
	}
	// get content of initial state
	err = py_rpc.FromGo(object[1])
	if err != nil {
		err = fmt.Errorf("Unmarshal: %v", err)
	}
	return
	// put data and return
}

// Output the object as json
func Debug(object any) (res string, err error) {
	str, err := json.MarshalIndent(object, "", "\t")
	if err != nil {
		err = fmt.Errorf("Debug: %v", err)
	}
	return string(str), err
}
