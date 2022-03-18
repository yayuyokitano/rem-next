package remblocklist

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
	functions.HTTP("blocklist", blocklist)
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
	GuildID   string `json:"guildID"`
	ChannelID string `json:"channelID"`
	Token     int64  `json:"token"`
	UserID    string `json:"userID"`
	ListType  string `json:"listType"`
	State     bool   `json:"state"`
}

func blocklist(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	var params Params

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.GuildID == "" || params.ChannelID == "" || params.Token == 0 || params.UserID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters")
		return
	}

	if err := confirmPermission(params.GuildID, params.UserID, params.Token); err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid permission: ", err)
		return
	}

	if err := pushToDB(params.GuildID, params.ChannelID, params.ListType, params.State, request); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to push to DB: ", err)
		return
	}

	if err := pushToRemraku(params.GuildID, params.ChannelID, params.ListType, params.State, request); err != nil {
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

func pushToDB(guildID string, channelID string, listType string, state bool, request *http.Request) (err error) {

	err = createPool()
	if err != nil {
		return
	}

	if !contains([]string{"xpgain"}, listType) {
		err = errors.New("Invalid list type")
		return
	}

	_, err = pool.Exec(request.Context(), fmt.Sprintf("INSERT INTO channelblocklist (guildID, channelID, %s) VALUES ($1, $2, $3) ON CONFLICT (channelID) DO UPDATE SET %s = $3", listType, listType), guildID, channelID, state)

	return

}

type blocklistMessage struct {
	GuildID   string `json:"guildID"`
	ChannelID string `json:"channelID"`
	ListType  string `json:"listType"`
	State     bool   `json:"state"`
}

func pushToRemraku(guildID string, channelID string, listType string, state bool, request *http.Request) (err error) {

	client, err = pubsub.NewClient(context.Background(), os.Getenv("GCP_PROJECT_ID"))
	if err != nil {
		return
	}

	pubsubRaw, err := json.Marshal(blocklistMessage{
		GuildID:   guildID,
		ChannelID: channelID,
		ListType:  listType,
		State:     state,
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
