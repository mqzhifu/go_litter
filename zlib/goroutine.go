package zlib

import "reflect"

type Goroutine struct {
	Log *Log
	Metrics *Metrics
}


func NewGoroutine(log *Log,metrics *Metrics)*Goroutine{
	self := new(Goroutine)
	self.Log = log
	self.Metrics = metrics
	return self
}

func (goroutine *Goroutine)  CreateExec(class interface{},funcNme string,argc ...interface{} ) {
	goroutine.Log.Warning("NewGoroutine ",funcNme," ,len: ",len(argc), " argc:",argc)
	inputs := make([]reflect.Value, len(argc))
	for i, _ := range argc {
		inputs[i] = reflect.ValueOf(argc[i])
	}

	myfunc := reflect.ValueOf(class).MethodByName(funcNme)
	//goroutine.Metrics.IncNode("goroutineNum")
	go myfunc.Call( inputs)
}