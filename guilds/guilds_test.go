package remguilds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGuilds(t *testing.T) {

	query := fmt.Sprintf(`/guilds?token=%s&userID=%s`, os.Getenv("REM_TEST_TOKEN"), os.Getenv("REM_TEST_USERID"))
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("GET", query, nil)
	guilds(writer, request)

	t.Log(writer.Body)

	var onboarded OnboardedGuilds
	if err := json.NewDecoder(writer.Body).Decode(&onboarded); err != nil {
		t.Error("Failed to decode service response: ", err)
		return
	}

	guildmap := compileGuilds(onboarded)

	expected := map[string]int{
		"719255152170762301": 2,
		"868947570603491338": 1,
		"716363971070001202": 0,
		"789384729388502839": 0,
	}

	for k, v := range expected {
		if guildmap[k] != v {
			t.Errorf("Guild %s: Expected %d, got %d", k, v, guildmap[k])
		}
	}

	//check that the guilds are sorted
	allTrue := true
	for _, guild := range onboarded {
		if guild.RemIsMember && !allTrue {
			t.Errorf("Rem is member of Guild %s yet a non-membered guild appeared before it", guild.Guild.ID)
		}
		allTrue = allTrue && guild.RemIsMember
	}

	query = fmt.Sprintf(`/guilds?token=%s&userID=%s}`, "789384729388502839", "789384729388502839")
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("GET", query, nil)
	guilds(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("Expected %d, got %d", http.StatusUnauthorized, writer.Code)
	}

}

func compileGuilds(onboarded OnboardedGuilds) (guildmap map[string]int) {

	guildmap = make(map[string]int)

	for _, guild := range onboarded {
		if guild.RemIsMember {
			guildmap[guild.Guild.ID] = 2
		} else {
			guildmap[guild.Guild.ID] = 1
		}
	}

	return
}
