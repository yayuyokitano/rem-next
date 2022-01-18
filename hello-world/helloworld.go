package remhelloworld

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

func init() {
	functions.HTTP("hello-world", helloWorld)
}

func helloWorld(writer http.ResponseWriter, request *http.Request) {

	var params struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		fmt.Fprint(writer, "Hello, World! (fail)")
		return
	}
	if params.Name == "" {
		fmt.Fprint(writer, "Hello, World! (empty)")
		return
	}
	fmt.Fprintf(writer, "Hello, %s", html.EscapeString(params.Name))

}
