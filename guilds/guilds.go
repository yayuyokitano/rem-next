package remguilds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4/pgxpool"
)

type discordError struct {
	Code             int    `json:"code"`
	Message          string `json:"message"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type Auth struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	discordError
}

type Token struct {
	UserID        string `datastore:"userID"`
	ExpiresAt     int64  `datastore:"expiresAt"`
	AccessToken   string `datastore:"accessToken"`
	RefreshToken  string `datastore:"refreshToken"`
	Username      string `datastore:"username"`
	Discriminator string `datastore:"discriminator"`
	Avatar        string `datastore:"avatar"`
}

type Guild struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Icon        string   `json:"icon"`
	IsOwner     bool     `json:"owner"`
	Permissions string   `json:"permissions"`
	Features    []string `json:"features"`
	discordError
}

type Guilds []Guild

type OnboardedGuild struct {
	Guild       Guild
	RemIsMember bool
}

type OnboardedGuilds []OnboardedGuild

var pool *pgxpool.Pool

func init() {
	functions.HTTP("guilds", guilds)
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

func guilds(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	urlParams := request.URL.Query()
	fmt.Fprint(writer, urlParams)
	token, err := strconv.ParseInt(urlParams.Get("token"), 10, 64)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Invalid token", err)
		return
	}

	params := struct {
		Token  int64
		UserID string
	}{
		Token:  token,
		UserID: urlParams.Get("userID"),
	}

	if params.Token == 0 || params.UserID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters")
		return
	}

	guilds, err := getGuildList(writer, params.Token, params.UserID)
	if err != nil {
		fmt.Fprint(writer, "Failed to fetch guild list", err)
		return
	}

	onboardedGuilds, err := checkOnboardedGuilds(writer, guilds)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to check onboarded guilds", err)
		return
	}

	jsonResponse, err := json.Marshal(onboardedGuilds)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal response", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, string(jsonResponse))

}

func getGuildList(writer http.ResponseWriter, token int64, userID string) (guilds Guilds, err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create datastore client: ")
		return
	}
	defer client.Close()

	var tokenData Token
	err = client.Get(ctx, datastore.IDKey("Token", token, nil), &tokenData)
	if err != nil || tokenData.UserID != userID {
		writer.WriteHeader(http.StatusUnauthorized)
		err = errors.New("Unable to authorize user")
		return
	}

	guilds, err = attemptFetchGuild(tokenData.AccessToken, tokenData.UserID)

	//try to refresh token and try again, if doesnt work then give up
	if err != nil {
		var auth Auth
		auth, err = refreshToken(tokenData.RefreshToken)
		if err != nil {
			writer.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(writer, "Failed to refresh token: ")
			return
		}

		_, err = client.Put(ctx, datastore.IDKey("Token", token, nil), &Token{
			UserID:        tokenData.UserID,
			ExpiresAt:     time.Now().Unix() + int64(auth.ExpiresIn),
			AccessToken:   auth.AccessToken,
			RefreshToken:  auth.RefreshToken,
			Username:      tokenData.Username,
			Discriminator: tokenData.Discriminator,
			Avatar:        tokenData.Avatar,
		})

		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(writer, "Failed to write refreshed token: ", err)
			return
		}

		guilds, err = attemptFetchGuild(auth.AccessToken, tokenData.UserID)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(writer, "Failed to fetch guild list: ", err)
		}
	}

	return
}

func attemptFetchGuild(token string, userID string) (guilds Guilds, err error) {
	baseUri := os.Getenv("DISCORD_BASE_URI")

	req, err := http.NewRequest("GET", baseUri+"/users/@me/guilds", nil)
	if err != nil {
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return
	}

	if len(guilds) == 0 {
		err = errors.New("No guilds found.")
	}
	return
}

func refreshToken(refreshToken string) (auth Auth, err error) {

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
		return
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&auth); err != nil {
		return
	}

	if auth.AccessToken == "" {
		err = errors.New("Failed to get access token from discord")
		return
	}

	return
}

func checkOnboardedGuilds(writer http.ResponseWriter, guilds Guilds) (onboarded OnboardedGuilds, err error) {

	if pool == nil {
		ctx := context.Background()

		pool, err = pgxpool.Connect(ctx, os.Getenv("DATABASE_PRIVATE_URL"))
		if err != nil {
			return
		}
	}

	cachedGuilds, err := pool.Query(context.Background(), "SELECT guildID FROM guilds WHERE guildID = ANY($1)", guilds.IDList())
	if err != nil {
		return
	}
	defer cachedGuilds.Close()

	guildIDMap := make(map[string]bool)

	for cachedGuilds.Next() {
		var guildID string
		err = cachedGuilds.Scan(&guildID)
		if err != nil {
			return
		}
		guildIDMap[guildID] = true
	}

	for _, guild := range guilds {
		onboarded = append(onboarded, OnboardedGuild{
			Guild:       guild,
			RemIsMember: guildIDMap[guild.ID],
		})
	}

	return

}

func (guilds Guilds) IDList() []string {
	idList := make([]string, len(guilds))
	for index, guild := range guilds {
		idList[index] = guild.ID
	}
	return idList
}
