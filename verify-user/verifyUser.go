package remverifyuser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	functions.HTTP("verify-user", verifyUser)
}

func verifyUser(writer http.ResponseWriter, request *http.Request) {

	writer.Header().Set("Access-Control-Allow-Origin", "https://rem.fm")

	var params struct {
		Token  int64  `json:"token"`
		UserID string `json:"userID"`
	}
	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}
	if params.Token == 0 || params.UserID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, "Missing parameters")
		return
	}

	token, err := confirmUser(writer, params.Token, params.UserID)
	if err != nil {
		return
	}

	jsonResponse, err := json.Marshal(TokenResponse{
		UserID:        token.UserID,
		Username:      token.Username,
		Discriminator: token.Discriminator,
		Avatar:        token.Avatar,
		Token:         params.Token,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to marshal response", err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	fmt.Fprint(writer, string(jsonResponse))

}

func confirmUser(writer http.ResponseWriter, token int64, userID string) (tokenData Token, err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create datastore client", err)
		return
	}
	defer client.Close()

	err = client.Get(ctx, datastore.IDKey("Token", token, nil), &tokenData)
	if err != nil || tokenData.UserID != userID {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Unable to authorize user")
		err = errors.New("Unable to authorize user")
		return
	}

	if tokenData.ExpiresAt > time.Now().Unix() {
		var auth AuthResponse
		auth, err = refreshToken(writer, tokenData.RefreshToken)
		if err != nil {
			return
		}

		var user UserResponse
		user, err = getUserInfo(writer, auth.AccessToken)
		if err != nil {
			return
		}

		err = updateToken(writer, user, auth, token)
		if err != nil {
			return
		}

		tokenData = Token{
			UserID:        user.ID,
			ExpiresAt:     time.Now().Unix() + int64(auth.ExpiresIn),
			AccessToken:   auth.AccessToken,
			RefreshToken:  auth.RefreshToken,
			Username:      user.Username,
			Discriminator: user.Discriminator,
			Avatar:        user.Avatar,
		}

	}

	return
}

func updateToken(writer http.ResponseWriter, userInfo UserResponse, authInfo AuthResponse, token int64) (err error) {

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create datastore client", err)
		return
	}
	defer client.Close()

	_, err = client.Put(ctx, datastore.IDKey("Token", token, nil), &Token{
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

	return

}

func getUserInfo(writer http.ResponseWriter, accessToken string) (userInfo UserResponse, err error) {

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

	var discordResponse UserResponse

	if err = json.NewDecoder(resp.Body).Decode(&discordResponse); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode discord response", err)
		return
	}

	if discordResponse.ID == "" {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to get user info from discord", discordResponse)
		err = errors.New("Failed to get user info from discord")
		return
	}

	return discordResponse, err

}

func refreshToken(writer http.ResponseWriter, refreshToken string) (authResponse AuthResponse, err error) {

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

	if err = json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode discord response", err)
		return
	}

	if authResponse.AccessToken == "" {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to get access token from discord", authResponse)
		err = errors.New("Failed to get access token from discord")
		return
	}

	return
}