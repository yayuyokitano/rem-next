package reminteraction

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInteractions(t *testing.T) {

	params := fmt.Sprintf(`{"name":"level", "defaultPermission":true, "guildID":"%s", "userID":"%s", "token":%s}`, os.Getenv("REM_TEST_GUILDID"), os.Getenv("REM_TEST_USERID"), os.Getenv("REM_TEST_TOKEN"))
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", "/interaction", strings.NewReader(params))

	addInteraction(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
	}

	commandID, err := getInteraction(os.Getenv("REM_TEST_GUILDID"), "level")
	if err != nil {
		t.Errorf("Failed to get interaction: %s\n", err)
	}

	if commandID == "" {
		t.Errorf("Failed to get interaction ID")
	}

	time.Sleep(5 * time.Second) //don't get rate limited

	writer = httptest.NewRecorder()
	request = httptest.NewRequest("DELETE", "/interaction", strings.NewReader(params))

	removeInteraction(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
	}

	commandID, err = getInteraction(os.Getenv("REM_TEST_GUILDID"), "level")
	if err == nil {
		t.Errorf("Expected error, got none")
	}

	paramsLackingPermissions := fmt.Sprintf(`{"name":"level", "defaultPermission":true, "guildID":"%s", "userID":"%s", "token":%s}`, os.Getenv("REM_TEST_GUILDID"), "267794154459889664", os.Getenv("REM_TEST_TOKEN"))
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("PUT", "/interaction", strings.NewReader(paramsLackingPermissions))

	addInteraction(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusUnauthorized, writer.Code, writer.Body)
	}

	time.Sleep(5 * time.Second) //don't get rate limited

	params = fmt.Sprintf(`{"name":"level", "defaultPermission":true, "guildID":"%s", "userID":"%s", "token":%s}`, os.Getenv("REM_TEST_GUILDID"), os.Getenv("REM_TEST_USERID"), os.Getenv("REM_TEST_TOKEN"))
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("PUT", "/interaction", strings.NewReader(params))

	addInteraction(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
	}

	commandID, err = getInteraction(os.Getenv("REM_TEST_GUILDID"), "level")
	if err != nil {
		t.Errorf("Failed to get interaction: %s\n", err)
	}

	if commandID == "" {
		t.Errorf("Failed to get interaction ID")
	}

}
