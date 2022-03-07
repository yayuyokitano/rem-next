package remconfirmpermission

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
)

func init() {
	functions.HTTP("confirm-permission", confirmPermission)
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

func confirmPermission(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	var params struct {
		UserID  string `json:"userID"`
		Token   int64  `json:"token"`
		GuildID string `json:"guildID"`
	}

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body: ", err)
		return
	}
	if params.UserID == "" || params.GuildID == "" || params.Token == 0 {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters.")
		return
	}

	if err := verifyUser(params.UserID, params.Token, params.GuildID); err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid token or user, or insufficient guild permissions: ", err)
		return
	}

	_, err := verifyGuild(params.GuildID)
	if err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Failed to verify guild: ", err)
		return
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

func verifyUser(userID string, Token int64, guildID string) (err error) {

	resp, err := http.Post(os.Getenv("GCP_BASE_URI")+"verify-user", "application/json", strings.NewReader(fmt.Sprintf(`{"userID":"%s","token":%d}`, userID, Token)))
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("verify-user returned status code %d", resp.StatusCode)
		return
	}
	var user TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return
	}
	resp.Body.Close()

	guildIDInt, err := strconv.ParseInt(guildID, 10, 64)
	if err != nil {
		return
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/users/@me/guilds?&after=%d&limit=1", os.Getenv("DISCORD_BASE_URI"), guildIDInt-1), nil)
	if err != nil {
		return
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.AccessToken))
	client := &http.Client{}

	resp, err = client.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("guild member returned status code %d", resp.StatusCode)
		return
	}

	var userPerms UserPerms

	err = json.NewDecoder(resp.Body).Decode(&userPerms)
	if err != nil {
		return
	}
	resp.Body.Close()

	Administrator := int64(8)
	perms, err := strconv.ParseInt(userPerms[0].Permissions, 10, 64)
	if err != nil {
		return
	}

	if perms&Administrator == 0 {
		err = fmt.Errorf("User does not have administrator permissions")
		return
	}

	return

}

type discordError struct {
	Code             int    `json:"code"`
	Message          string `json:"message"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type Auth struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	discordError
}

type GuildID struct {
	ID string `json:"id"`
}

type Guild struct {
	discordError
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresIn    int     `json:"expires_in"`
	GuildID      GuildID `json:"guild"`
}

type Token struct {
	ExpiresAt    int64  `datastore:"expiresAt"`
	AccessToken  string `datastore:"accessToken"`
	RefreshToken string `datastore:"refreshToken"`
}

func verifyGuild(guildID string) (tokenData Token, err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()

	err = client.Get(ctx, datastore.NameKey("Guild", guildID, nil), &tokenData)
	if err != nil {
		return
	}

	// Refresh token if expired, else just fetch updated guild data

	if tokenData.ExpiresAt < time.Now().Unix() {

		var auth Auth
		auth, err = refreshToken(tokenData.RefreshToken)
		if err != nil {
			return
		}

		tokenData = Token{
			ExpiresAt:    time.Now().Unix() + int64(auth.ExpiresIn),
			AccessToken:  auth.AccessToken,
			RefreshToken: auth.RefreshToken,
		}

	}

	err = updateToken(tokenData, guildID)
	if err != nil {
		return
	}

	tokenData = Token{
		ExpiresAt:    tokenData.ExpiresAt,
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
	}

	return
}

func updateToken(tokenData Token, guildID string) (err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()

	_, err = client.Put(ctx, datastore.NameKey("Guild", guildID, nil), &Token{
		ExpiresAt:    tokenData.ExpiresAt,
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
	})

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
