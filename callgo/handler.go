package callgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
)

type registerOptions struct {
	securityToken string
	logger        *slog.Logger
}

type RegisterOption func(r *registerOptions)

func WithSecurityToken(token string) RegisterOption {
	return func(r *registerOptions) {
		r.securityToken = token
	}
}

func WithLogger(logger *slog.Logger) RegisterOption {
	return func(r *registerOptions) {
		r.logger = logger
	}
}

func Register(opts ...RegisterOption) {
	var cnf registerOptions
	for _, opt := range opts {
		opt(&cnf)
	}

	if cnf.logger == nil {
		cnf.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	http.HandleFunc("/__callgo", callHandler(cnf))
}

type invokeRequest struct {
	FnName         string          `json:"fnname"`
	Accountability *Accountability `json:"accountability"`
	Payload        json.RawMessage `json:"payload"`
	Trigger        *RawTrigger     `json:"trigger"`
}

func callHandler(cnf registerOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cnf.securityToken != "" {
			if r.Header.Get("Authorization") != "Bearer "+cnf.securityToken {
				http.Error(w, "wrong authorization token", http.StatusUnauthorized)
				return
			}
		}

		var ir invokeRequest
		if err := json.NewDecoder(r.Body).Decode(&ir); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		cnf.logger.InfoContext(r.Context(), "Function called", slog.String("fnname", ir.FnName))

		f, ok := funcs[ir.FnName]
		if !ok {
			http.Error(w, fmt.Sprintf("function %q not found", ir.FnName), http.StatusNotFound)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, accountabilityKey, ir.Accountability)
		ctx = context.WithValue(ctx, rawTriggerKey, ir.Trigger)

		args := []reflect.Value{
			reflect.ValueOf(ctx),
		}
		if f.fv.Type().NumIn() == 2 {
			payload := reflect.New(f.fv.Type().In(1).Elem())
			if err := json.Unmarshal(ir.Payload, payload.Interface()); err != nil {
				cnf.logger.ErrorContext(r.Context(), "callgo: cannot decode request payload",
					slog.String("fnname", ir.FnName),
					slog.String("error", err.Error()),
					slog.String("payload", string(ir.Payload)),
					slog.String("target", fmt.Sprintf("%T", payload.Interface())))
				http.Error(w, fmt.Sprintf("cannot decode request payload: %s", err), http.StatusBadRequest)
				return
			}
			args = append(args, payload)
		}

		out := f.fv.Call(args)
		switch len(out) {
		case 1:
			if err := out[0].Interface(); err != nil {
				emitUserError(cnf, r, w, ir, err.(error))
				return
			}
			fmt.Fprintln(w, "{}")

		case 2:
			if err := out[1].Interface(); err != nil {
				emitUserError(cnf, r, w, ir, err.(error))
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(out[0].Interface()); err != nil {
				http.Error(w, fmt.Sprintf("cannot encode response data: %s", err), http.StatusInternalServerError)
				return
			}

		default:
			panic("should not reach here")
		}
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func emitUserError(cnf registerOptions, r *http.Request, w http.ResponseWriter, ir invokeRequest, err error) {
	cnf.logger.ErrorContext(r.Context(), "callgo: function call error",
		slog.String("error", err.Error()),
		slog.String("fnname", ir.FnName))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(errorResponse{Error: fmt.Sprint(err)}); err != nil {
		http.Error(w, fmt.Sprintf("cannot encode error response: %s", err), http.StatusInternalServerError)
		return
	}
}
