package remauthorizediscord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"cloud.google.com/go/datastore"
	"gopkg.in/h2non/gock.v1"
)

type UserDetails struct {
	UserID       string `firestore:"userID"`
	AccessToken  string `firestore:"accessToken"`
	RefreshToken string `firestore:"refreshToken"`
	ExpiresAt    int64  `firestore:"expiresAt"`
}

func TestAuthorizeDiscord(t *testing.T) {

	baseUri := os.Getenv("DISCORD_BASE_URI")
	defer gock.DisableNetworking()

	gock.EnableNetworking()
	gock.New(baseUri).
		Post("/oauth2/token").
		Reply(200).
		JSON(map[string]interface{}{
			"access_token":  "lZAR5LqvY8d8vOVTwxxsMhxeNGYriW",
			"expires_in":    604800,
			"refresh_token": "IBX2JcNQPWmZYwrqXwFMkEr1CCgd6R",
			"scope":         "identify guilds",
			"token_type":    "Bearer",
		})

	params := "{\"code\":\"qnUH03bY6qLiz67Rh95CvfA7cWEc0t\"}"
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/authorize-discord", strings.NewReader(params))

	authorizeDiscord(writer, request)

	var tokenResponse TokenResponse
	if err := json.NewDecoder(writer.Body).Decode(&tokenResponse); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode service response", err)
		return
	}

	if tokenResponse.Token == 0 {
		t.Error("Received empty token")
	}

	expected := TokenResponse{
		UserID:        "196249128286552064",
		Username:      "Themex",
		Discriminator: "2404",
		Avatar:        "ce27b9e198424fc276a6072a02459a37",
		Token:         tokenResponse.Token,
	}

	if !reflect.DeepEqual(tokenResponse, expected) {
		t.Errorf("Expected %v, got %v", expected, tokenResponse)
	}

	gock.Off()

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		t.Error("Failed to create datastore client", err)
	}
	defer client.Close()

	err = client.Delete(ctx, datastore.IDKey("Token", tokenResponse.Token, nil))

	if err != nil {
		t.Error("Failed to delete token", err)
	}

	params = "{\"code\":\"qnUH03bY6qSiz67Rh95CvfA7cWEc0t\"}"
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/authorize-discord", strings.NewReader(params))
	expectedString := "Failed to get access token from discord{  0   {0  invalid_request Invalid \"code\" in request.}}"

	authorizeDiscord(writer, request)
	if writer.Body.String() != expectedString {
		t.Errorf("Expected %s, got %s", expectedString, writer.Body.String())
	}
}
