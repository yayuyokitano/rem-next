package reminteraction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4/pgxpool"
)

func init() {
	functions.HTTP("interaction", interaction)
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

func interaction(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

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
	if err := confirmPermissions(params.UserID, params.Token, params.GuildID); err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid token or user, or insufficient guild permissions: ", err)
		return
	}

	switch request.Method {
	case "POST":
		if len(params.SubCommands) == 0 {
			removeInteraction(params, writer)
		} else {
			addInteraction(params, writer)
		}
		break
	case "PATCH":
		modifyPermissions(params, writer)
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

type UserPerms []struct {
	Permissions string `json:"permissions"`
}

type InteractionParams struct {
	Name              string       `json:"name"`
	SubCommands       []string     `json:"subCommands"`
	DefaultPermission bool         `json:"defaultPermission"`
	GuildID           string       `json:"guildID"`
	UserID            string       `json:"userID"`
	Token             int64        `json:"token"`
	Permissions       []Permission `json:"permissions"`
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

	ctx := context.Background()
	_, err = pool.Exec(ctx, "DELETE FROM commands WHERE commandID = $1", commandDetails.ID)
	_, err = pool.Exec(ctx, "DELETE FROM commands WHERE guildID = $1 AND commandName = $2", commandDetails.GuildID, commandDetails.Name)
	_, err = pool.Exec(ctx, "INSERT INTO commands (commandID, guildID, commandName) VALUES ($1, $2, $3)", commandDetails.ID, commandDetails.GuildID, commandDetails.Name)
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

type ConfirmPermissionParams struct {
	UserID  string `json:"userID"`
	Token   int64  `json:"token"`
	GuildID string `json:"guildID"`
}

func addInteraction(params InteractionParams, writer http.ResponseWriter) {

	interaction, err := createInteraction(params.Name, params.SubCommands, params.DefaultPermission)

	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Interaction does not exist: ", err)
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
	if commandID != "" {
		commandID = "/" + commandID
	}
	var requestMethod string
	if isPatch {
		requestMethod = "PATCH"
	} else {
		requestMethod = "POST"
	}

	interactionURL := fmt.Sprintf("%s/applications/%s/guilds/%s/commands%s", os.Getenv("DISCORD_BASE_URI"), os.Getenv("DISCORD_CLIENT_ID"), params.GuildID, commandID)

	req, err := http.NewRequest(requestMethod, interactionURL, strings.NewReader(string(interactionJSON)))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create request", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", os.Getenv("DISCORD_TOKEN")))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to send request", err)
		return
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		writer.WriteHeader(http.StatusInternalServerError)
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Fprint(writer, "Failed to create interaction", resp.StatusCode, string(respBody))
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

func removeInteraction(params InteractionParams, writer http.ResponseWriter) {

	commandID, err := getInteraction(params.GuildID, params.Name)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Interaction does not exist: ", err)
		return
	}

	interactionURL := fmt.Sprintf("%s/applications/%s/guilds/%s/commands/%s", os.Getenv("DISCORD_BASE_URI"), os.Getenv("DISCORD_CLIENT_ID"), params.GuildID, commandID)

	req, err := http.NewRequest("DELETE", interactionURL, nil)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create request", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", os.Getenv("DISCORD_TOKEN")))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to send request", err)
		return
	}
	if resp.StatusCode != http.StatusNoContent {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to delete interaction", resp.StatusCode, err)
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

func modifyPermissions(params InteractionParams, writer http.ResponseWriter) {

	commandID, err := getInteraction(params.GuildID, params.Name)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Interaction does not exist: ", err)
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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", os.Getenv("DISCORD_TOKEN")))

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

func confirmPermissions(userID string, token int64, guildID string) (err error) {
	b, err := json.Marshal(ConfirmPermissionParams{
		UserID:  userID,
		Token:   token,
		GuildID: guildID,
	})
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", os.Getenv("GCP_BASE_URI")+"confirm-permission", bytes.NewReader(b))
	if err != nil {
		return
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Failed to confirm permissions: %s", resp.Status)
	}
	return
}
