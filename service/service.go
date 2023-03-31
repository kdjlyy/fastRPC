package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// ================================
// 通过反射实现结构体与服务的映射关系
// ================================

// MethodType 实例包含了一个方法的完整信息
// func (t *T) MethodName(argType T1, replyType *T2) error
type MethodType struct {
	method    reflect.Method // 方法本身
	ArgType   reflect.Type   // 客户端参数（值或指针类型）
	ReplyType reflect.Type   // 服务端返回的数据（指针类型）
	numCalls  uint64         // 统计方法调用次数时会用到
}

func (m *MethodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

// NewArgv 和 NewReplyv，用于创建对应类型的实例
func (m *MethodType) NewArgv() reflect.Value {
	var argv reflect.Value

	// arg may be a pointer type, or a value type
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *MethodType) NewReplyv() reflect.Value {
	// reply must be a pointer type
	replyv := reflect.New(m.ReplyType.Elem())

	// m.ReplyType.Elem() = Array, Chan, Map, Ptr, or Slice
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}

// =================================================

// Service format "Service.Method", e.g. "Foo.Sum"
// 'Foo' is name, 'Method' is method.
type Service struct {
	this   reflect.Value          // 结构体的实例本身,调 用时需要 this 作为第 0 个参数
	name   string                 // 映射的结构体的名称，如 WaitGroup
	typ    reflect.Type           // 结构体的类型
	method map[string]*MethodType // 存储映射的结构体的所有符合条件的方法 {"MethodName": *methodType}
}

func (s *Service) GetName() string                      { return s.name }
func (s *Service) GetType() reflect.Type                { return s.typ }
func (s *Service) GetMethodMap() map[string]*MethodType { return s.method }
func (s *Service) GetMethod(name string) *MethodType    { return s.method[name] }

// NewService 构造函数
func NewService(this interface{}) *Service {
	s := new(Service)
	s.this = reflect.ValueOf(this)
	s.name = reflect.Indirect(s.this).Type().Name() // reflect.Indirect 会返回指针所指向的值
	s.typ = reflect.TypeOf(this)

	// 判断一个标识符是否是导出的
	if !ast.IsExported(s.name) {
		log.Fatalf("FastRPC server: %s is not a valid service name", s.name)
	}

	s.registerMethods()

	return s
}

// registerMethods 过滤出了符合条件的方法
// func (t *T) MethodName(argType T1, replyType *T2) error
// 1. 两个导出或内置类型的入参（反射时为 3 个，第 0 个是自身，类似于 python 的 self，C++ 中的 this）
// 2. 返回值有且只有 1 个，类型为 error
func (s *Service) registerMethods() {
	s.method = make(map[string]*MethodType)

	// reflect.Type.NumMethod 返回一个类型的方法集中方法的数量
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type

		// 因为NumIn()包括this、argType、replyType，NumOut()为error
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}

		// reflect.Type.Out 返回函数类型的输出参数类型列表，列表里应该只有一个error类型
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}

		s.method[method.Name] = &MethodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("FastRPC server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

// Call 能够通过反射值调用方法
func (s *Service) Call(m *MethodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)

	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.this, argv, replyv})

	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}

	return nil
}
