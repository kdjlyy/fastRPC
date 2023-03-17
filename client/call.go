package client

// Call represents an active RPC.
// 封装了结构体 Call 来承载一次 RPC 调用所需要的信息
type Call struct {
	Seq           uint64
	ServiceMethod string      // format "<service>.<method>"
	Args          interface{} // arguments to the function
	Reply         interface{} // reply from the function
	Error         error       // if error occurs, it will be set

	// 1. 为了支持异步调用，当调用结束时，Client会调用 call.done() 通知调用方
	// 2. 当前RPC调用还未完成时，Client出现故障，Client会调用 call.done() 通知调用方
	Done chan *Call // Strobes when call is complete.
}

func (call *Call) done() {
	call.Done <- call
}
