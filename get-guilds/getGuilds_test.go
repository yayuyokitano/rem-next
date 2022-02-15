package remgetguilds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGetGuilds(t *testing.T) {

	params := fmt.Sprintf(`{"token":%s,"userID":"%s"}`, os.Getenv("REM_TEST_TOKEN"), os.Getenv("REM_TEST_USERID"))
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/get-guilds", strings.NewReader(params))
	getGuilds(writer, request)

	var guildmap map[int64]int
	if err := json.NewDecoder(writer.Body).Decode(&guildmap); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode service response", err)
		return
	}

	expected := map[int64]int{
		719255152170762301: 2,
		716363971070001202: 1,
		789384729388502839: 0,
	}

	for k, v := range expected {
		if guildmap[k] != v {
			t.Errorf("Guild %d: Expected %d, got %d", k, v, guildmap[k])
		}
	}

	params = fmt.Sprintf(`{"token":%s,"userID":"%s"}`, "789384729388502839", "789384729388502839")
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/get-guilds", strings.NewReader(params))
	getGuilds(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("Expected %d, got %d", http.StatusUnauthorized, writer.Code)
	}

}

func compileGuilds(onboarded OnboardedGuilds) (guildmap map[int64]int) {

	guildmap = make(map[int64]int)

	for _, guild := range onboarded {
		if guild.RemIsMember {
			guildmap[guild.Guild.ID] = 2
		} else {
			guildmap[guild.Guild.ID] = 1
		}
	}

	return
}
