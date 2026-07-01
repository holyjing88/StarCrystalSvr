package api

import (
	"context"
	"net/http"
)

type idipCtxKey struct{}

func withIdipUsername(r *http.Request, username string) *http.Request {
	ctx := context.WithValue(r.Context(), idipCtxKey{}, username)
	return r.WithContext(ctx)
}

func idipUserFrom(r *http.Request) string {
	if r == nil {
		return ""
	}
	u, _ := r.Context().Value(idipCtxKey{}).(string)
	return u
}
