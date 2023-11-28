package route

import (
	"context"
	"strings"
	"testing"
	server2 "wailsrouter/pkg/app/server"
)

var fakeHandlerValue string

type testRequests []struct {
	path       string
	nilHandler bool
	route      string
	ps         server2.Params
}

func getParams() *server2.Params {
	ps := make(server2.Params, 0, 20)
	return &ps
}

func checkRequests(t *testing.T, tree *RadixTree, requests testRequests, unescapes ...bool) {
	unescape := false
	if len(unescapes) >= 1 {
		unescape = unescapes[0]
	}

	for _, request := range requests {
		params := getParams()
		value := tree.find(request.path, params, unescape)
		// 实际查不到
		if value.handlers == nil {
			// 实际查不到 预想是查得到 打印错误
			if !request.nilHandler {
				t.Errorf("handle mismatch for route '%s': Expected non-nil handle", request.path)
			}
		} else
		// 实际查到 预想是查不到 打印错误
		if request.nilHandler {
			t.Errorf("handle mismatch for route '%s': Expected nil handle", request.path)
		} else
		// 实际查到 预想是查得到
		{
			// 执行handler
			value.handlers[0](context.Background(), nil)
			// fakeHandlerValue 应该和 request.route 一致
			// 不一致说明 handler 查找的不对
			if fakeHandlerValue != request.route {
				t.Errorf("handle mismatch for route '%s': Wrong handle (%s != %s)", request.path, fakeHandlerValue, request.route)
			}
		}
		// 检查参数
		for _, item := range request.ps {
			if item.Value != (*params).ByName(item.Key) {
				t.Errorf("mismatch params. path: %s, key: %s, expected value: %s, actual value: %s", request.path, item.Key, item.Value, (*params).ByName(item.Key))
			}
		}
	}
}

// fakeHandler 返回一个只有一个handler的HandlersChain
func fakeHandler(val string) server2.HandlersChain {
	return server2.HandlersChain{func(c context.Context, ctx *server2.RequestContext) {
		fakeHandlerValue = val
	}}
}

// catchPanic 捕捉函数参数的panic
func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

// TestTreeAddAndGet 注册静态路由查找 测试
func TestTreeAddAndGet(t *testing.T) {
	// 创建路由树
	tree := &RadixTree{root: &node{}}
	// 路由规则的key
	routes := [...]string{
		"/hi",
		"/contact",
		"/co",
		"/c",
		"/a",
		"/ab",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/α",
		"/β",
	}
	// 注册完整的路由规则
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	// 查询测试
	checkRequests(t, tree, testRequests{
		{"", true, "", nil},                  // 查不到
		{"a", true, "", nil},                 // 查不到
		{"/", true, "", nil},                 // 查不到
		{"/con", true, "", nil},              // key mismatch
		{"/cona", true, "", nil},             // key mismatch
		{"/no", true, "", nil},               // no matching child
		{"/a", false, "/a", nil},             // 查到
		{"/hi", false, "/hi", nil},           // 查到
		{"/contact", false, "/contact", nil}, // 查到
		{"/co", false, "/co", nil},           // 查到
		{"/ab", false, "/ab", nil},           // 查到
		{"/α", false, "/α", nil},             // 查到
		{"/β", false, "/β", nil},             // 查到
	})
}

// 注册动态路由查找(包含 : *) 测试
func TestTreeWildcard(t *testing.T) {
	tree := &RadixTree{root: &node{}}

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/cmd/xxx/",
		"/src/*filepath",
		"/search/",
		"/search/:query",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir/*filepath",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
		"/a/b/:c",
		"/a/:b/c/d",
		"/a/*b",
	}
	// 添加路由规则
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}

	checkRequests(t, tree, testRequests{
		{"/cmd/test", true, "", nil},                     // 查不到
		{"/search/someth!ng+in+ünìcodé/", true, "", nil}, // 查不到
		{"/", false, "/", nil},                           // 查到
		{"/cmd/test/", false, "/cmd/:tool/", server2.Params{server2.Param{Key: "tool", Value: "test"}}},                                             // 查到
		{"/cmd/test/3", false, "/cmd/:tool/:sub", server2.Params{server2.Param{Key: "tool", Value: "test"}, server2.Param{Key: "sub", Value: "3"}}}, // 查到
		{"/src/", false, "/src/*filepath", server2.Params{server2.Param{Key: "filepath", Value: ""}}},                                               // 查到
		{"/src/some/file.png", false, "/src/*filepath", server2.Params{server2.Param{Key: "filepath", Value: "some/file.png"}}},                     // 查到
		{"/search/", false, "/search/", nil}, // 查到
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", server2.Params{server2.Param{Key: "query", Value: "someth!ng+in+ünìcodé"}}},                                             // 查到 		// 查不到
		{"/user_gopher", false, "/user_:name", server2.Params{server2.Param{Key: "name", Value: "gopher"}}},                                                                               // 查到
		{"/user_gopher/about", false, "/user_:name/about", server2.Params{server2.Param{Key: "name", Value: "gopher"}}},                                                                   // 查到
		{"/files/js/inc/framework.js", false, "/files/:dir/*filepath", server2.Params{server2.Param{Key: "dir", Value: "js"}, server2.Param{Key: "filepath", Value: "inc/framework.js"}}}, // 查到
		{"/info/gordon/public", false, "/info/:user/public", server2.Params{server2.Param{Key: "user", Value: "gordon"}}},                                                                 // 查到
		{"/info/gordon/project/go", false, "/info/:user/project/:project", server2.Params{server2.Param{Key: "user", Value: "gordon"}, server2.Param{Key: "project", Value: "go"}}},       // 查到
		{"/a/b/c", false, "/a/b/:c", server2.Params{server2.Param{Key: "c", Value: "c"}}},                                                                                                 // 查到
		{"/a/b/c/d", false, "/a/:b/c/d", server2.Params{server2.Param{Key: "b", Value: "b"}}},                                                                                             // 查到
		{"/a/b", false, "/a/*b", server2.Params{server2.Param{Key: "b", Value: "b"}}},                                                                                                     // 查到
	})
}

// 重复路由规则注册 测试
func TestTreeDuplicatePath(t *testing.T) {
	tree := &RadixTree{root: &node{}}

	routes := [...]string{
		"/",
		"/doc/",
		"/src/*filepath",
		"/search/:query",
		"/user_:name",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}

		// Add again 重复添加 应该捕捉到panic
		recv = catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		// 没有捕捉到panic 打印错误
		if recv == nil {
			t.Fatalf("no panic while inserting duplicate route '%s", route)
		}
	}

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},         // 查到
		{"/doc/", false, "/doc/", nil}, // 查到
		{"/src/some/file.png", false, "/src/*filepath", server2.Params{server2.Param{Key: "filepath", Value: "some/file.png"}}},               // 查到
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", server2.Params{server2.Param{Key: "query", Value: "someth!ng+in+ünìcodé"}}}, // 查到
		{"/user_gopher", false, "/user_:name", server2.Params{server2.Param{Key: "name", Value: "gopher"}}},                                   // 查到
	})
}

// 空通配符名称 测试
func TestEmptyWildcardName(t *testing.T) {
	tree := &RadixTree{root: &node{}}
	// 这些路由规则注册应该捕捉到panic(规则不合法
	routes := [...]string{
		"/user:",
		"/user:/",
		"/cmd/:/",
		"/src/*",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})
		// 没有捕捉到panic 打印错误
		if recv == nil {
			t.Fatalf("no panic while inserting route with empty wildcard name '%s", route)
		}
	}
}

// 规则支持的最大参数 测试
func TestTreeCatchMaxParams(t *testing.T) {
	tree := &RadixTree{root: &node{}}
	route := "/cmd/*filepath"
	tree.addRoute(route, fakeHandler(route))
}

// 路由规则只允许一个通配符
func TestTreeDoubleWildcard(t *testing.T) {
	const panicMsg = "only one wildcard per path segment is allowed"
	// 这些路由规则注册应该捕捉到panic(规则不合法)
	// 每个路径段仅允许使用一个通配符 : *
	routes := [...]string{
		"/:foo:bar",
		"/:foo:bar/",
		"/:foo*bar",
	}

	for _, route := range routes {
		tree := &RadixTree{root: &node{}}
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})
		// 如果没有捕捉到panic 或则 panic信息不符合预期 打印错误
		if rs, ok := recv.(string); !ok || !strings.HasPrefix(rs, panicMsg) {
			t.Fatalf(`"Expected panic "%s" for route '%s', got "%v"`, panicMsg, route, recv)
		}
	}
}

// 重定向
func TestTreeTrailingSlashRedirect2(t *testing.T) {
	tree := &RadixTree{root: &node{}}

	routes := [...]string{
		"/api/:version/seller/locales/get",
		"/api/v:version/seller/permissions/get",
		"/api/v:version/seller/university/entrance_knowledge_list/get",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		// 如果捕捉到panic 打印错误
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}
	v := make(server2.Params, 0, 1)

	// 测试尾斜杠重定向
	// 比注册的路由规则少一个斜杠 但是查找的时候会自动重定向
	tsrRoutes := [...]string{
		"/api/v:version/seller/permissions/get/",
		"/api/version/seller/permissions/get/",
	}
	// 查询的结果应该是 handlers为空 tsr为true
	for _, route := range tsrRoutes {
		value := tree.find(route, &v, false)
		// 如果查找到的路由规则不为空 打印错误
		if value.handlers != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else
		// handler为空 并且 tsr为false 打印错误
		if !value.tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/api/v:version/seller/permissions/get/a",
	}
	// 查询的结果应该是 handler为空 tsr为false
	for _, route := range noTsrRoutes {
		value := tree.find(route, &v, false)
		// 如果查找到的路由规则不为空 打印错误
		if value.handlers != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else
		// handler为空 并且 tsr为true 打印错误
		if value.tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

// 重定向
func TestTreeTrailingSlashRedirect(t *testing.T) {
	tree := &RadixTree{root: &node{}}

	routes := [...]string{
		"/hi",
		"/b/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/admin",
		"/admin/:category",
		"/admin/:category/:page",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/no/a",
		"/no/b",
		"/api/hello/:name",
		"/user/:name/*id",
		"/resource",
		"/r/*id",
		"/book/biz/:name",
		"/book/biz/abc",
		"/book/biz/abc/bar",
		"/book/:page/:name",
		"/book/hello/:name/biz/",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		// 如果捕捉到panic 打印错误
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}
	// 需要重定向的路由规则对树中的字符串多一个斜杠或者少一个斜杠
	tsrRoutes := [...]string{
		"/hi/",
		"/b",
		"/search/gopher/",
		"/cmd/vet",
		"/src",
		"/x/",
		"/y",
		"/0/go/",
		"/1/go",
		"/a",
		"/admin/",
		"/admin/config/",
		"/admin/config/permissions/",
		"/doc/",
		"/user/name",
		"/r",
		"/book/hello/a/biz",
		"/book/biz/foo/",
		"/book/biz/abc/bar/",
	}
	v := make(server2.Params, 0, 10)
	for _, route := range tsrRoutes {
		value := tree.find(route, &v, false)
		if value.handlers != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else if !value.tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/",
		"/no",
		"/no/",
		"/_",
		"/_/",
		"/api/world/abc",
		"/book",
		"/book/",
		"/book/hello/a/abc",
		"/book/biz/abc/biz",
	}
	for _, route := range noTsrRoutes {
		value := tree.find(route, &v, false)
		if value.handlers != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else if value.tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

// 重定向
func TestTreeRootTrailingSlashRedirect(t *testing.T) {
	tree := &RadixTree{root: &node{}}

	recv := catchPanic(func() {
		tree.addRoute("/:test", fakeHandler("/:test"))
	})
	// 如果捕捉到panic 打印错误
	if recv != nil {
		t.Fatalf("panic inserting test route: %v", recv)
	}

	// 预期应该是重定向到 /:test handler不为空 tsr为true
	value := tree.find("/", nil, false)
	// 如果查找到的路由规则不为空 打印错误
	if value.handlers != nil {
		t.Fatalf("non-nil handler")
	} else
	// handler为空 并且 tsr为true 打印错误
	if value.tsr {
		t.Errorf("expected no TSR recommendation")
	}
}

// 参数测试
func TestTreeParamNotOptimize(t *testing.T) {
	tree := &RadixTree{root: &node{}}
	routes := [...]string{
		"/:parama/start",
		"/:paramb",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	checkRequests(t, tree, testRequests{
		{"/1", false, "/:paramb", server2.Params{server2.Param{Key: "paramb", Value: "1"}}},             // 查到
		{"/1/start", false, "/:parama/start", server2.Params{server2.Param{Key: "parama", Value: "1"}}}, // 查到
	})

	// other sequence
	tree = &RadixTree{root: &node{}}
	routes = [...]string{
		"/:paramb",
		"/:parama/start",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	checkRequests(t, tree, testRequests{
		{"/1/start", false, "/:parama/start", server2.Params{server2.Param{Key: "parama", Value: "1"}}}, // 查到
		{"/1", false, "/:paramb", server2.Params{server2.Param{Key: "paramb", Value: "1"}}},             // 查到
	})
}
