package main

import (
	"context"
	"fmt"
	"github.com/Yuki-J1/wailsrouter/pkg/app/server"
	"github.com/Yuki-J1/wailsrouter/pkg/route"
	"time"
)

func main() {
	de := route.NewEngine()
	de.Handle("/", HandlerTest1)
	de.Handle("/user/:name", HandlerTest1)

	v1group := de.Group("v1")
	{
		v1group.Handle("/user", HandlerTest1)
		v1group.Handle("/login", HandlerTest1)
	}

	requestCtx := de.NewContext()
	requestCtx.Path = []byte("/user/YKJ")
	de.Serve(context.Background(), requestCtx)

	time.Sleep(100 * time.Second)
}

func HandlerTest1(c context.Context, ctx *server.RequestContext) {
	val, ok := ctx.Params.Get("name")
	if ok == true {
		fmt.Print("handlerTest1")
		fmt.Print(val)
	}
}
