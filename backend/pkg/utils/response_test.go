package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccessAndErrorResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	SuccessResponse(rec, http.StatusCreated, map[string]int{"n": 1})
	if rec.Code != 201 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("code=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var ok struct {
		Status string         `json:"status"`
		Data   map[string]int `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ok)
	if ok.Status != "success" || ok.Data["n"] != 1 {
		t.Fatalf("%+v", ok)
	}

	rec = httptest.NewRecorder()
	ErrorResponse(rec, http.StatusBadRequest, "nope", map[string]string{"f": "bad"})
	var er struct {
		Status  string            `json:"status"`
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &er)
	if rec.Code != 400 || er.Status != "error" || er.Message != "nope" || er.Details["f"] != "bad" {
		t.Fatalf("code=%d %+v", rec.Code, er)
	}
}
