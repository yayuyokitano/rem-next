package reminteraction

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4/pgxpool"
)

func init() {
	functions.HTTP("level", level)
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

var pool *pgxpool.Pool

func level(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	switch request.Method {
	case "DELETE":
		removeInteraction(writer, request)
		break
	case "PUT":
		addInteraction(writer, request)
		break
	case "PATCH":
		modifyPermissions(writer, request)
		break
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(writer, "Method not allowed")
		break
	}

}

type TokenResponse struct {
	UserID        string `json:"userID"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Token         int64  `json:"token"`
	AccessToken   string `json:"accessToken"`
}

type User struct {
	Token  int64  `json:"token"`
	UserID string `json:"userID"`
}

type UserPerms struct {
	Permissions string `json:"permissions"`
}

func verifyUser(userID string, Token int64, guildID string) bool {

	resp, err := http.Post(os.Getenv("GCP_BASE_URI")+"verify-user", "application/json", strings.NewReader(fmt.Sprintf(`{"userID":"%s","token":%d}`, userID, Token)))
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	var user TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return false
	}
	resp.Body.Close()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/users/@me/guilds/%s/member", os.Getenv("DISCORD_BASE_URI"), guildID), nil)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.AccessToken))
	client := &http.Client{}

	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}

	var userPerms UserPerms

	err = json.NewDecoder(resp.Body).Decode(&userPerms)
	if err != nil {
		return false
	}
	resp.Body.Close()

	Administrator := int64(8)
	perms, err := strconv.ParseInt(userPerms.Permissions, 10, 64)
	if err != nil {
		return false
	}

	if perms&Administrator == 0 {
		return false
	}

	return true

}

type InteractionParams struct {
	Name              string `json:"name"`
	DefaultPermission bool   `json:"defaultPermission"`
	GuildID           string `json:"guildID"`
	UserID            string `json:"userID"`
	Token             int64  `json:"token"`
}

type CommandDetails struct {
	Name    string `json:"name"`
	GuildID string `json:"guild_id"`
	ID      string `json:"id"`
}

func createPool() (err error) {
	if pool == nil {
		ctx := context.Background()

		pool, err = pgxpool.Connect(ctx, os.Getenv("DATABASE_PRIVATE_URL"))
	}
	return
}

func storeInteraction(commandDetails CommandDetails) (err error) {

	err = createPool()
	if err != nil {
		return
	}

	_, err = pool.Exec(context.Background(), "INSERT INTO commands (commandID, guildID, commandName) VALUES ($1, $2, $3)", commandDetails.ID, commandDetails.GuildID, commandDetails.Name)
	return

}

func getInteraction(guildID string, commandName string) (commandID string, err error) {
	err = createPool()
	if err != nil {
		return
	}

	row := pool.QueryRow(context.Background(), "SELECT commandID FROM commands WHERE guildID = $1 AND commandName = $2", guildID, commandName)
	err = row.Scan(&commandID)
	return
}

func deleteInteraction(commandID string) (err error) {
	err = createPool()
	if err != nil {
		return
	}

	_, err = pool.Exec(context.Background(), "DELETE FROM commands WHERE commandID = $1", commandID)
	return
}

func addInteraction(writer http.ResponseWriter, request *http.Request) {

	var params InteractionParams

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.Name == "" || params.GuildID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters.")
		return
	}

	if !verifyUser(params.UserID, params.Token, params.GuildID) {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid token or user, or insufficient guild permissions.")
		return
	}

	interaction, err := createInteraction(params.Name, params.DefaultPermission)

	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Interaction does not exist.", err)
		return
	}

	interactionJSON, err := json.Marshal(interaction)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal response", err)
		return
	}

	commandID, err := getInteraction(params.GuildID, params.Name)
	isPatch := err == nil && commandID != ""
	if !isPatch {
		commandID = ""
	}
	var requestMethod string
	if isPatch {
		requestMethod = "PATCH"
	} else {
		requestMethod = "POST"
	}

	interactionURL := fmt.Sprintf("%s/applications/%s/guilds/%s/commands/%s", os.Getenv("DISCORD_BASE_URI"), os.Getenv("DISCORD_CLIENT_ID"), params.GuildID, commandID)

	req, err := http.NewRequest(requestMethod, interactionURL, strings.NewReader(string(interactionJSON)))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create request", err)
		return
	}

	token, err := verifyGuild(params.GuildID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to verify guild", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to send request", err)
		return
	}

	if resp.StatusCode != http.StatusCreated {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create interaction", err)
		return
	}

	var commandDetails CommandDetails
	err = json.NewDecoder(resp.Body).Decode(&commandDetails)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode response", err)
		return
	}

	resp.Body.Close()

	err = storeInteraction(commandDetails)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to store interaction", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, "Successfully added interaction.")

}

func removeInteraction(writer http.ResponseWriter, request *http.Request) {

	var params InteractionParams

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.Name == "" || params.GuildID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters.")
		return
	}

	if !verifyUser(params.UserID, params.Token, params.GuildID) {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid token or user, or insufficient guild permissions.")
		return
	}

	commandID, err := getInteraction(params.GuildID, params.Name)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Interaction does not exist.", err)
		return
	}

	interactionURL := fmt.Sprintf("%s/applications/%s/guilds/%s/commands/%s", os.Getenv("DISCORD_BASE_URI"), os.Getenv("DISCORD_CLIENT_ID"), params.GuildID, commandID)

	req, err := http.NewRequest("DELETE", interactionURL, nil)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create request", err)
		return
	}

	token, err := verifyGuild(params.GuildID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to verify guild", err)
		return
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to send request", err)
		return
	}
	if resp.StatusCode != http.StatusNoContent {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to delete interaction", err)
		return
	}

	err = deleteInteraction(commandID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to delete interaction from REM, there may be issues joomS", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, "Successfully removed interaction.")

}

type PermissionParams struct {
	GuildID     string       `json:"guild_id"`
	Name        string       `json:"name"`
	UserID      string       `json:"user_id"`
	Token       int64        `json:"token"`
	Permissions []Permission `json:"permissions"`
}

func modifyPermissions(writer http.ResponseWriter, request *http.Request) {

	var params PermissionParams

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.Name == "" || params.GuildID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters.")
		return
	}

	if !verifyUser(params.UserID, params.Token, params.GuildID) {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid token or user, or insufficient guild permissions.")
		return
	}

	commandID, err := getInteraction(params.GuildID, params.Name)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Interaction does not exist.", err)
		return
	}

	interactionURL := fmt.Sprintf("%s/applications/%s/guilds/%s/commands/%s/permissions", os.Getenv("DISCORD_BASE_URI"), os.Getenv("DISCORD_CLIENT_ID"), params.GuildID, commandID)

	permissionsJSON, err := json.Marshal(params.Permissions)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal permissions", err)
		return
	}

	req, err := http.NewRequest("PUT", interactionURL, strings.NewReader(string(permissionsJSON)))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create request", err)
		return
	}

	token, err := verifyGuild(params.GuildID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to verify guild", err)
		return
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to send request", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to modify permissions", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, "Successfully modified permissions of interaction.")

}
