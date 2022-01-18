package remhelloworld

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHelloWorld(t *testing.T) {
	params := "{\"name\":\"rem\"}"
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/hello-world", strings.NewReader(params))
	expected := "Hello, re"

	helloWorld(writer, request)
	if writer.Body.String() != expected {
		t.Errorf("Expected %s, got %s", expected, writer.Body.String())
	}
}
