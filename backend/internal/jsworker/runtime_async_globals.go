package jsworker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

type runtimeAsyncGlobalHooks struct {
	StructuredClone func(value goja.Value) (goja.Value, error)
	QueueMicrotask  func(callback goja.Callable) error
	SetTimeout      func(callback goja.Callable, delay time.Duration, args []goja.Value) (int64, error)
	ClearTimeout    func(id int64) bool
}

func installRuntimeAsyncCompatibilityGlobals(vm *goja.Runtime, hooks runtimeAsyncGlobalHooks) {
	if vm == nil {
		return
	}
	if hooks.StructuredClone != nil {
		_ = vm.Set("structuredClone", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) == 0 {
				return goja.Undefined()
			}
			cloned, err := hooks.StructuredClone(call.Arguments[0])
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return cloned
		})
	}
	if hooks.QueueMicrotask != nil {
		_ = vm.Set("queueMicrotask", func(call goja.FunctionCall) goja.Value {
			callback, err := resolveRuntimeAsyncCallback(vm, call, "queueMicrotask")
			if err != nil {
				panic(vm.NewGoError(err))
			}
			if err := hooks.QueueMicrotask(callback); err != nil {
				panic(vm.NewGoError(err))
			}
			return goja.Undefined()
		})
	}
	if hooks.SetTimeout != nil {
		_ = vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
			callback, err := resolveRuntimeAsyncCallback(vm, call, "setTimeout")
			if err != nil {
				panic(vm.NewGoError(err))
			}
			args := make([]goja.Value, 0, max(len(call.Arguments)-2, 0))
			if len(call.Arguments) > 2 {
				args = append(args, call.Arguments[2:]...)
			}
			id, err := hooks.SetTimeout(callback, runtimeAsyncDelayFromGojaValue(argumentAt(call, 1)), args)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(id)
		})
	}
	if hooks.ClearTimeout != nil {
		_ = vm.Set("clearTimeout", func(call goja.FunctionCall) goja.Value {
			value := argumentAt(call, 0)
			if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
				return vm.ToValue(false)
			}
			return vm.ToValue(hooks.ClearTimeout(value.ToInteger()))
		})
	}
}

func resolveRuntimeAsyncCallback(vm *goja.Runtime, call goja.FunctionCall, name string) (goja.Callable, error) {
	if vm == nil {
		return nil, fmt.Errorf("%s is unavailable", strings.TrimSpace(name))
	}
	if len(call.Arguments) == 0 {
		return nil, fmt.Errorf("%s(callback) requires callback", strings.TrimSpace(name))
	}
	callback, ok := goja.AssertFunction(call.Arguments[0])
	if !ok || callback == nil {
		return nil, fmt.Errorf("%s(callback) requires a callable function", strings.TrimSpace(name))
	}
	return callback, nil
}

func runtimeAsyncDelayFromGojaValue(value goja.Value) time.Duration {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return 0
	}
	delay := value.ToInteger()
	if delay < 0 {
		delay = 0
	}
	return time.Duration(delay) * time.Millisecond
}

func cloneRuntimeStructuredValue(value interface{}) (interface{}, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("structuredClone only supports JSON-serializable values: %w", err)
	}
	var cloned interface{}
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return nil, fmt.Errorf("structuredClone decode failed: %w", err)
	}
	return cloned, nil
}

func runtimeStructuredCloneValue(vm *goja.Runtime, value goja.Value) (goja.Value, error) {
	if vm == nil {
		return goja.Undefined(), fmt.Errorf("runtime is unavailable")
	}
	if value == nil || goja.IsUndefined(value) {
		return goja.Undefined(), nil
	}
	if goja.IsNull(value) {
		return goja.Null(), nil
	}
	cloned, err := cloneRuntimeStructuredValue(value.Export())
	if err != nil {
		return nil, err
	}
	return runtimeBridgeValue(vm, cloned), nil
}
