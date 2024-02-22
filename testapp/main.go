package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/altipla-consulting/directus-call-go/callgo"
)

var noParamsNoReturnFn = callgo.Func("NoParamsNoReturn", func(ctx context.Context) error {
	fmt.Println("NoParamsNoReturn called")
	return nil
})

var noParamsWithReturnFn = callgo.Func("NoParamsWithReturn", func(ctx context.Context) (string, error) {
	return "foo-value", nil
})

type fooExample struct {
	Foo string `json:"foo"`
	Bar int32  `json:"bar"`
}

var paramWithReturnFn = callgo.Func("ParamWithReturn", func(ctx context.Context, foo *fooExample) (*fooExample, error) {
	foo.Foo += "new-foo-value"
	foo.Bar = 42
	return foo, nil
})

var accountabilityFn = callgo.Func("Accountability", func(ctx context.Context) error {
	fmt.Printf("%#v\n", callgo.AccountabilityFromContext(ctx))
	return nil
})

var errorFn = callgo.Func("Error", func(ctx context.Context) error {
	return fmt.Errorf("error message")
})

func main() {
	callgo.Register(callgo.WithLogger(slog.Default()))

	slog.Info("Test server listening on :8080")
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		slog.Error("Cannot start server", "error", err)
		os.Exit(1)
	}
}
