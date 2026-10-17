package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type signupReq struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=2,max=20"`
}

func TestBindAndValidate(t *testing.T) {
	cases := []struct {
		name, body string
		wantOK     bool
		wantStatus int
	}{
		{"valid", `{"email":"a@b.co","name":"Sam"}`, true, 200},
		{"bad json", `{`, false, 400},
		{"unknown field", `{"email":"a@b.co","name":"Sam","x":1}`, false, 400},
		{"trailing data", `{"email":"a@b.co","name":"Sam"}{}`, false, 400},
		{"invalid fields", `{"email":"nope","name":"S"}`, false, 422},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.body))
			var req signupReq
			ok := BindAndValidate(w, r, &req)
			if ok != c.wantOK {
				t.Fatalf("ok=%v want %v", ok, c.wantOK)
			}
			if !ok && w.Code != c.wantStatus {
				t.Fatalf("status=%d want %d body=%s", w.Code, c.wantStatus, w.Body)
			}
		})
	}
	if errs := Validate(&signupReq{Email: "x", Name: ""}); errs["email"] == "" || errs["name"] == "" {
		t.Fatalf("expected json-named field errors, got %v", errs)
	}
}
