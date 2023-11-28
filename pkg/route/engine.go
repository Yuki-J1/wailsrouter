package route

import (
	"context"
	"fmt"
	"github.com/Yuki-J1/wailsrouter/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"sync"
)

type Engine struct {
	RouterGroup
	tree         RadixTree
	PanicHandler server.HandlerFunc
	ctxPool      sync.Pool
	maxParams    uint16
}

func NewEngine() *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{
			Handlers: nil,  // 根路由组 handler 为nil
			basePath: "/",  // 根路由组  basePath default is "/"
			root:     true, // 表示是否为根路由组
		},
		tree:      RadixTree{root: &node{}},
		maxParams: 64,
	}
	engine.RouterGroup.engine = engine
	engine.ctxPool.New = func() interface{} {
		ctx := engine.NewContext()
		return ctx
	}
	return engine
}

func (engine *Engine) NewContext() *server.RequestContext {
	return server.NewContext(engine.maxParams)
}

// addRoute 直接通过 func (r *RadixTree) addRoute 添加路由
func (engine *Engine) addRoute(path string, handlers server.HandlersChain) {
	// path必须不为空 否则panic
	if len(path) == 0 {
		panic("path should not be ''")
	}
	// path必须/字符开头 否则panic
	utils.Assert(path[0] == '/', "path must begin with '/'")
	// handlers必须不能为空 handlers
	utils.Assert(len(handlers) > 0, "there must be at least one handler")

	// 添加路由
	engine.tree.addRoute(path, handlers)
}

func (engine *Engine) Serve(c context.Context, ctx *server.RequestContext) {
	// 用于防止服务器因未恢复的恐慌而崩溃。
	if engine.PanicHandler != nil {
		defer engine.recv(ctx)
	}

	// path
	rPath := string(ctx.Path)

	// 查询
	paramsPointer := &ctx.Params
	value := engine.tree.find(rPath, paramsPointer, false)
	if value.handlers != nil {
		// 为请求上下文设置handlers
		ctx.SetHandlers(value.handlers)
		// 为请求上下文设置path
		ctx.SetFullPath(value.fullPath)
		// 开始进入洋葱
		ctx.Next(c)
		return
	}

	defaultError(c, ctx)
}

func (engine *Engine) recv(ctx *server.RequestContext) {
	// 如果捕获到panic, 使用PanicHandler处理
	if rcv := recover(); rcv != nil {
		engine.PanicHandler(context.Background(), ctx)
	}
}

// defaultError 默认错误处理
func defaultError(c context.Context, ctx *server.RequestContext) {
	//	返回路由不到
	fmt.Print("没找到")
}
