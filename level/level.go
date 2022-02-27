package remlevel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

func init() {
	functions.HTTP("level", level)
}

type OptionChoice struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Option struct {
	Type         int            `json:"type"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Required     bool           `json:"required"`
	Choices      []OptionChoice `json:"choices"`
	Options      []Option       `json:"options"`
	ChannelTypes []int          `json:"channel_types"`
	MinValue     float64        `json:"min_value"`
	MaxValue     float64        `json:"max_value"`
	Autocomplete bool           `json:"autocomplete"`
}

type Interaction struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Options            []Option `json:"options"`
	DefaultInteraction bool     `json:"default_interaction"`
	Type               int      `json:"type"`
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func corsHandler(writer http.ResponseWriter, request *http.Request) {
	allowed := strings.Split(os.Getenv("ALLOWED_ORIGINS"), "||")
	origin := request.Header.Get("Origin")
	if contains(allowed, origin) {
		writer.Header().Set("Access-Control-Allow-Origin", origin)
	}
}

func level(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	switch request.Method {
	case "POST":
		runLevelInteraction(writer, request)
		break
	case "PATCH":
		modifyLevelPermissions(writer, request)
		break
	case "DELETE":
		removeLevelInteraction(writer, request)
		break
	case "PUT":
		addLevelInteraction(writer, request)
		break
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(writer, "Method not allowed")
		break
	}

}

type LevelInteraction struct {
	DefaultPermission bool     `json:"defaultPermission"`
	Type              int      `json:"type"`
	Roles             []string `json:"options"`
	GuildID           string   `json:"guildID"`
}

func modifyLevelPermissions(writer http.ResponseWriter, request *http.Request) {
}

func addLevelInteraction(writer http.ResponseWriter, request *http.Request) {

	var params LevelInteraction

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.Type == 0 {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters.")
		return
	}

	optionUser := 6
	displayOptions := make([]Option, 1)
	displayOptions = append(displayOptions, Option{
		Type:        optionUser,
		Name:        "user",
		Description: "The user to show the level of, defaults to yourself.",
	})

	subCommand := 1
	levelOptions := make([]Option, 1)
	levelOptions = append(levelOptions, Option{
		Type:        subCommand,
		Name:        "display",
		Description: "Display the level of a user.",
		Options:     displayOptions,
	})

	interaction, err := json.Marshal(Interaction{
		Name:               "level",
		Description:        "Show level or level leaderboard related things.",
		Options:            levelOptions,
		DefaultInteraction: true,
		Type:               1,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal response", err)
		return
	}

	interactionURL := fmt.Sprintf("%s/applications/%s/guilds/%s/commands", os.Getenv("DISCORD_BASE_URI"), os.Getenv("DISCORD_CLIENT_ID"), params.GuildID)

	//create a post request and send the json to discord
	req, err := http.NewRequest("POST", interactionURL, strings.NewReader(string(interaction)))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create request", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("DISCORD_TOKEN")))

}

func removeLevelInteraction(writer http.ResponseWriter, request *http.Request) {

}

func runLevelInteraction(writer http.ResponseWriter, request *http.Request) {
	var params struct {
		Token        string   `json:"token"`
		UserID       string   `json:"userID"`
		GuildID      string   `json:"guildID"`
		Interactions []string `json:"interactions"`
	}
	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.Token == "" || params.UserID == "" || params.GuildID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters.")
		return
	}

	/*jsonResponse, err := json.Marshal(TokenResponse{
		UserID:        userInfo.ID,
		Username:      userInfo.Username,
		Discriminator: userInfo.Discriminator,
		Avatar:        userInfo.Avatar,
		Token:         token,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal response", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, string(jsonResponse))*/
}
