package route

import (
	"github.com/Yuki-J1/wailsrouter/pkg/app/server"
	"path"
)

type IRouter interface {
	IRoutes
	Group(string, ...server.HandlerFunc) *RouterGroup
}

type IRoutes interface {
	Use(...server.HandlerFunc) IRoutes
	Handle(string, ...server.HandlerFunc) IRoutes
}

var _ IRouter = (*RouterGroup)(nil)

type RouterGroup struct {
	Handlers server.HandlersChain // 基础handler 中间件
	basePath string               // 基础path
	engine   *Engine
	root     bool
}

// region ========== Use ==========

// Use 向路由组添加基础handler(中间件middleware)
func (group *RouterGroup) Use(middleware ...server.HandlerFunc) IRoutes {
	group.Handlers = append(group.Handlers, middleware...)
	return group.returnObj()
}

// endregion

// region ========== Handle ==========
func (group *RouterGroup) Handle(relativePath string, handlers ...server.HandlerFunc) IRoutes {
	return group.handle(relativePath, handlers)
}

func (group *RouterGroup) handle(relativePath string, handlers server.HandlersChain) IRoutes {
	// 整合 完整路径
	absolutePath := group.calculateAbsolutePath(relativePath)
	// 整合 完整handlers
	handlers = group.combineHandlers(handlers)
	// 添加路由
	group.engine.addRoute(absolutePath, handlers)
	return group.returnObj()
}

// 组合AbsolutePath
func (group *RouterGroup) calculateAbsolutePath(relativePath string) string {
	return joinPaths(group.basePath /* RouterGroup基础路径 */, relativePath /* 相对路径 */)
}

func joinPaths(absolutePath, relativePath string) string {
	// 相对路径为空 返回RouterGroup基础路径
	if relativePath == "" {
		return absolutePath
	}
	// eg: absolutePath = "/"  relativePath = "/user/name"
	finalPath := path.Join(absolutePath, relativePath)
	appendSlash := lastChar(relativePath) == '/' && lastChar(finalPath) != '/'
	if appendSlash {
		return finalPath + "/"
	}

	return finalPath
}

func lastChar(str string) uint8 {
	if str == "" {
		panic("The length of the string can't be 0")
	}
	return str[len(str)-1]
}

// 组合Handlers
func (group *RouterGroup) combineHandlers(handlers server.HandlersChain) server.HandlersChain {
	// = 当前路由组已有的handler长度 + 此次需要组合的handler长度
	// 计算组合后的大小
	finalSize := len(group.Handlers) + len(handlers)
	// 限制handlers中handler个数最多为63个
	if finalSize >= 63 {
		panic("too many handlers")
	}
	// 创建指定大小 server.HandlersChain 类型 实例
	mergedHandlers := make(server.HandlersChain, finalSize)
	// 先将 路由组已有的handler 添加到 handlers
	copy(mergedHandlers, group.Handlers)
	// 再将 此次需要组合的handler 添加到 handlers
	copy(mergedHandlers[len(group.Handlers):], handlers)
	return mergedHandlers
}

// endregion

// region ========== Group ==========

// Group 根据当前路由组的基础路径、基础handler 创建新的子路由组。
func (group *RouterGroup) Group(relativePath string, handlers ...server.HandlerFunc) *RouterGroup {
	return &RouterGroup{
		Handlers: group.combineHandlers(handlers),
		basePath: group.calculateAbsolutePath(relativePath),
		engine:   group.engine,
	}
}

// endregion

func (group *RouterGroup) returnObj() IRoutes {
	if group.root {
		return group.engine
	}
	return group
}
