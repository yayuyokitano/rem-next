package remauthorizeguild

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
	GuildID      string `datastore:"guildID"`
	ExpiresAt    int64  `datastore:"expiresAt"`
	AccessToken  string `datastore:"accessToken"`
	RefreshToken string `datastore:"refreshToken"`
}

type TokenResponse struct {
	GuildID string `json:"guildID"`
}

func init() {
	functions.HTTP("authorize-guild", authorizeGuild)
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

func authorizeGuild(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	var params struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.Code == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "No authorization code specified")
		return
	}

	guild, err := getGuildInfo(writer, params.Code)
	if err != nil {
		return
	}

	err = createToken(writer, guild)
	if err != nil {
		return
	}

	jsonResponse, err := json.Marshal(TokenResponse{
		GuildID: guild.Guild.ID,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal response", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, string(jsonResponse))

}

func getGuildInfo(writer http.ResponseWriter, code string) (auth GuildResponse, err error) {

	baseUri := os.Getenv("DISCORD_BASE_URI")
	clientID := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_SECRET")
	redirectURI := os.Getenv("DISCORD_REDIRECT_URI")

	formData := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI + "/authorization"},
		"code":          {code},
	}

	resp, err := http.PostForm(baseUri+"/oauth2/token", formData)

	if err != nil {
		writer.WriteHeader(http.StatusFailedDependency)
		fmt.Fprint(writer, "Discord request failed", err)
		return
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&auth); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode discord response", err)
		return
	}

	if auth.AccessToken == "" {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to get access token from discord", auth)
		err = errors.New("Failed to get access token from discord")
		return
	}

	return
}

func createToken(writer http.ResponseWriter, guild GuildResponse) (err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create datastore client", err)
		return
	}
	defer client.Close()

	_, err = client.Put(ctx, datastore.NameKey("Guild", guild.Guild.ID, nil), &Token{
		ExpiresAt:    time.Now().Unix() + int64(guild.ExpiresIn),
		AccessToken:  guild.AccessToken,
		RefreshToken: guild.RefreshToken,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to write token", err)
		return
	}

	return

}
