package remverifyguild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

type discordError struct {
	Code             int    `json:"code"`
	Message          string `json:"message"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type Guild struct {
	ID string `json:"id"`
}

type GuildResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Guild        Guild  `json:"guild"`
	discordError
}

type Token struct {
	ExpiresAt    int64  `datastore:"expiresAt"`
	AccessToken  string `datastore:"accessToken"`
	RefreshToken string `datastore:"refreshToken"`
}

type TokenResponse struct {
	GuildID     string `json:"guildID"`
	AccessToken string `json:"accessToken"`
}

func init() {
	functions.HTTP("verify-guild", verifyGuild)
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

func verifyGuild(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	var params struct {
		GuildID string `json:"guildID"`
	}
	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.GuildID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters")
		return
	}

	token, err := confirmGuild(writer, params.GuildID)
	if err != nil {
		return
	}

	jsonResponse, err := json.Marshal(TokenResponse{
		GuildID:     params.GuildID,
		AccessToken: token.AccessToken,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal response", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, string(jsonResponse))

}

func confirmGuild(writer http.ResponseWriter, guildID string) (tokenData Token, err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create datastore client", err)
		return
	}
	defer client.Close()

	err = client.Get(ctx, datastore.NameKey("Guild", guildID, nil), &tokenData)
	if err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Unable to find guild")
		err = errors.New("Unable to find guild")
		return
	}

	// Refresh token if expired

	if tokenData.ExpiresAt < time.Now().Unix() {

		var guild GuildResponse
		guild, err = refreshToken(writer, tokenData.RefreshToken)
		if err != nil {
			return
		}

		tokenData = Token{
			ExpiresAt:    time.Now().Unix() + int64(guild.ExpiresIn),
			AccessToken:  guild.AccessToken,
			RefreshToken: guild.RefreshToken,
		}

		err = updateToken(writer, tokenData, guildID)

	}

	return
}

func updateToken(writer http.ResponseWriter, tokenData Token, guildID string) (err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create datastore client", err)
		return
	}
	defer client.Close()

	_, err = client.Put(ctx, datastore.NameKey("Guild", guildID, nil), tokenData)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to write token", err)
	}

	return

}

func refreshToken(writer http.ResponseWriter, refreshToken string) (guild GuildResponse, err error) {

	baseUri := os.Getenv("DISCORD_BASE_URI")
	clientID := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_SECRET")

	formData := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm(baseUri+"/oauth2/token", formData)

	if err != nil {
		writer.WriteHeader(http.StatusFailedDependency)
		fmt.Fprint(writer, "Discord request failed", err)
		return
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&guild); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode discord response", err)
		return
	}

	if guild.AccessToken == "" {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to get access token from discord")
		err = errors.New("Failed to get access token from discord")
	}

	return
}
