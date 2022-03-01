package reminteraction

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	"cloud.google.com/go/datastore"
)

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