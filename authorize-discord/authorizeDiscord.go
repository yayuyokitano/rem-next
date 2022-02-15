package remauthorizediscord

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

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	discordError
}

type UserResponse struct {
	discordError
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	PublicFlags   int    `json:"public_flags"`
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

type TokenResponse struct {
	UserID        string `json:"userID"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Token         int64  `json:"token"`
}

func init() {
	functions.HTTP("authorize-discord", authorizeDiscord)
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

func authorizeDiscord(writer http.ResponseWriter, request *http.Request) {

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

	authInfo, err := getAuthInfo(writer, params.Code)
	if err != nil {
		return
	}

	userInfo, err := getUserInfo(writer, authInfo.AccessToken)
	if err != nil {
		return
	}

	token, err := createToken(writer, userInfo, authInfo)
	if err != nil {
		return
	}

	jsonResponse, err := json.Marshal(TokenResponse{
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
	fmt.Fprint(writer, string(jsonResponse))

}

func getAuthInfo(writer http.ResponseWriter, code string) (auth AuthResponse, err error) {

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

func getUserInfo(writer http.ResponseWriter, accessToken string) (user UserResponse, err error) {

	baseUri := os.Getenv("DISCORD_BASE_URI")

	req, err := http.NewRequest("GET", baseUri+"/users/@me", nil)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create request to discord", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		writer.WriteHeader(http.StatusFailedDependency)
		fmt.Fprint(writer, "Discord request failed", err)
		return
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&user); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode discord response", err)
		return
	}

	if user.ID == "" {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to get user info from discord", user)
		err = errors.New("Failed to get user info from discord")
		return
	}

	return

}

func createToken(writer http.ResponseWriter, userInfo UserResponse, authInfo AuthResponse) (token int64, err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create datastore client", err)
		return
	}
	defer client.Close()

	tokenKey, err := client.Put(ctx, datastore.IncompleteKey("Token", nil), &Token{
		UserID:        userInfo.ID,
		ExpiresAt:     time.Now().Unix() + int64(authInfo.ExpiresIn),
		AccessToken:   authInfo.AccessToken,
		RefreshToken:  authInfo.RefreshToken,
		Username:      userInfo.Username,
		Discriminator: userInfo.Discriminator,
		Avatar:        userInfo.Avatar,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to write token", err)
		return
	}

	token = tokenKey.ID

	return

}
