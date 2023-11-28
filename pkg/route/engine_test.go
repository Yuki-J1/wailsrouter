package route

import (
	"context"
	"fmt"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"testing"
	"wailsrouter/pkg/app/server"
)

func TestNew_Engine(t *testing.T) {
	router := NewEngine()
	assert.DeepEqual(t, "/", router.basePath)
	assert.DeepEqual(t, router.engine, router)
	assert.DeepEqual(t, 0, len(router.Handlers))
}

func TestEngine_Routes(t *testing.T) {
	de := NewEngine()
	de.handle("/", server.HandlersChain{HandlerTest1})
	de.handle("/user/:name", server.HandlersChain{HandlerTest2})

	v1group := de.Group("v1")
	{
		v1group.handle("/user", server.HandlersChain{HandlerTest1})
		v1group.handle("/login", server.HandlersChain{HandlerTest2})
	}

	requestCtx := de.NewContext()
	requestCtx.Path = []byte("/user/YKJ")
	de.Serve(context.Background(), requestCtx)
}
func HandlerTest1(c context.Context, ctx *server.RequestContext) {
	fmt.Print("handlerTest1")
}
func HandlerTest2(c context.Context, ctx *server.RequestContext) {}
