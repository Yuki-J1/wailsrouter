package server

import (
	"context"
	"sync"
)

// HandlersChain defines a HandlerFunc array.
type HandlersChain []HandlerFunc

type HandlerFunc func(c context.Context, ctx *RequestContext)

type RequestContext struct {
	Params   Params
	handlers HandlersChain // 查询到的处理函数
	fullPath string        // 查询到的路由规则
	mu       sync.RWMutex
	Keys     map[string]interface{}
	index    int8 // 调用链指针
	Path     []byte
}

func NewContext(maxParams uint16) *RequestContext {
	v := make(Params, 0, maxParams)
	ctx := &RequestContext{Params: v, index: -1}
	return ctx
}

func (ctx *RequestContext) SetHandlers(hc HandlersChain) {
	ctx.handlers = hc
}

func (ctx *RequestContext) SetFullPath(p string) {
	ctx.fullPath = p
}

func (ctx *RequestContext) Next(c context.Context) {
	// ctx.index 指向当前执行的handler
	// index++ 表示指针指向下一个handler
	// ctx.index 初始值为-1，所以第一次执行时，index++ 下标指向0 第一个handler
	ctx.index++
	for ctx.index < int8(len(ctx.handlers)) {
		// 执行下一个handler
		ctx.handlers[ctx.index](c, ctx)
		// 如果当前handler执行完毕，那么index会自增
		ctx.index++
	}
}
