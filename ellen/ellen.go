package ellen

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
)

func Printf(name string, args map[string]interface{}, result map[string]interface{}) {
	callerPC, callerFile, callerLine, _ := runtime.Caller(3)
	callerFile = filepath.Base(callerFile)
	callerFunName := filepath.Base(runtime.FuncForPC(callerPC).Name())

	calleePC, _, _, _ := runtime.Caller(2)
	//calleeFile = filepath.Base(calleeFile)
	calleeFunName := filepath.Base(runtime.FuncForPC(calleePC).Name())

	argsBytes, _ := json.Marshal(args)
	resultBytes, _ := json.Marshal(result)

	fmt.Printf("Calling %s from %s at %s:%d, arguments:%s, returns:%s\n", calleeFunName, callerFunName, callerFile, callerLine, string(argsBytes), string(resultBytes))
}
