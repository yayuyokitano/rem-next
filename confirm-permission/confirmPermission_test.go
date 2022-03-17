package remconfirmpermission

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestConfirmPermission(t *testing.T) {
	params := fmt.Sprintf(`{"guildID":"%s", "userID":"%s", "token":%s}`, os.Getenv("REM_TEST_GUILDID"), os.Getenv("REM_TEST_USERID"), os.Getenv("REM_TEST_TOKEN"))
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/confirm-permission", strings.NewReader(params))

	confirmPermission(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
	}

	paramsLackingPermissions := fmt.Sprintf(`{"guildID":"%s", "userID":"%s", "token":%s}`, os.Getenv("REM_TEST_GUILDID"), "267794154459889664", os.Getenv("REM_TEST_TOKEN"))
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/confirm-permission", strings.NewReader(paramsLackingPermissions))

	confirmPermission(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusUnauthorized, writer.Code, writer.Body)
	}

	paramsLackingPermissions = fmt.Sprintf(`{"guildID":"%s", "userID":"%s", "token":%s}`, "868012888827256882", os.Getenv("REM_TEST_USERID"), os.Getenv("REM_TEST_TOKEN"))
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/confirm-permission", strings.NewReader(paramsLackingPermissions))

	confirmPermission(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusUnauthorized, writer.Code, writer.Body)
	}

}
