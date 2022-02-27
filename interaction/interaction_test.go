package reminteraction

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func __TestAddInteraction(t *testing.T) {

	params := fmt.Sprintf(`{"name":"level", "defaultPermission":true, "guildID":"%s", "userID":"%s", "token":%s}`, os.Getenv("REM_TEST_GUILDID"), os.Getenv("REM_TEST_USERID"), os.Getenv("REM_TEST_TOKEN"))
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", "/interaction", strings.NewReader(params))

	addInteraction(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
	}

}
