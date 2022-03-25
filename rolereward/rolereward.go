package remrolereward

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4/pgxpool"
)

func init() {
	functions.HTTP("rolereward", roleReward)
}

var client *pubsub.Client

var pool *pgxpool.Pool

func createPool() (err error) {
	if pool == nil {
		ctx := context.Background()

		pool, err = pgxpool.Connect(ctx, os.Getenv("DATABASE_PRIVATE_URL"))
	}
	return
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

type Params struct {
	GuildID    string `json:"guildID"`
	RoleID     string `json:"roleID"`
	Token      int64  `json:"token"`
	UserID     string `json:"userID"`
	Level      int    `json:"level"`
	Persistent bool   `json:"persistent"`
	State      bool   `json:"state"`
}

func roleReward(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	var params Params

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.GuildID == "" || params.RoleID == "" || params.Token == 0 || params.UserID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters")
		return
	}

	if err := confirmPermission(params.GuildID, params.UserID, params.Token); err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid permission: ", err)
		return
	}

	if err := pushToDB(params.GuildID, params.RoleID, params.Level, params.Persistent, params.State, request); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to push to DB: ", err)
		return
	}

	if err := pushToRemraku(params.GuildID, params.RoleID, params.Level, params.Persistent, params.State, request); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to push to Remraku: ", err)
		return
	}

}

func confirmPermission(guildID string, callerID string, token int64) (err error) {

	client := &http.Client{}
	resp, err := client.Post(os.Getenv("GCP_BASE_URI")+"/confirm-permission", "application/json", strings.NewReader(fmt.Sprintf(`{"guildID":"%s","userID":"%s","token":%d}`, guildID, callerID, token)))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = errors.New("Invalid token or user, or insufficient guild permissions: " + string(rawBody))
		return
	}
	return
}

func pushToDB(guildID string, roleID string, level int, persistent bool, state bool, request *http.Request) (err error) {

	err = createPool()
	if err != nil {
		return
	}

	if state {
		_, err = pool.Exec(request.Context(), "INSERT INTO roleRewards (guildID, roleID, level, persistent) VALUES ($1, $2, $3, $4) ON CONFLICT (guildID, roleID, level) DO UPDATE SET persistent = $4", guildID, roleID, level, persistent)
	} else {
		_, err = pool.Exec(request.Context(), "DELETE FROM roleRewards WHERE guildID = $1 AND roleID = $2 AND level = $3", guildID, roleID, level)
	}
	return

}

type rolerewardMessage struct {
	Type       string `json:"type"`
	GuildID    string `json:"guildID"`
	RoleID     string `json:"roleID"`
	Level      int    `json:"level"`
	Persistent bool   `json:"persistent"`
	State      bool   `json:"state"`
}

func pushToRemraku(guildID string, roleID string, level int, persistent bool, state bool, request *http.Request) (err error) {

	client, err = pubsub.NewClient(context.Background(), os.Getenv("GCP_PROJECT_ID"))
	if err != nil {
		return
	}

	pubsubRaw, err := json.Marshal(rolerewardMessage{
		Type:       "rolereward",
		GuildID:    guildID,
		RoleID:     roleID,
		Level:      level,
		Persistent: persistent,
		State:      state,
	})
	if err != nil {
		return
	}

	m := &pubsub.Message{
		Data: pubsubRaw,
	}

	_, err = client.Topic("remraku").Publish(request.Context(), m).Get(request.Context())

	return

}
