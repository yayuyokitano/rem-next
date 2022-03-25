package remconfirmpermission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

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

	err := verifyGuild(params.GuildID, params.Token)
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
		err = errors.New("Failed to verify user: " + err.Error())
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

func verifyGuild(guildID string, token int64) (err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	defer client.Close()

	var tokenData Token

	err = client.Get(ctx, datastore.NameKey("Guild", guildID, nil), &tokenData)
	if err != nil {
		return
	}

	if tokenData.AccessToken == "" {
		err = errors.New("No token found")
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
