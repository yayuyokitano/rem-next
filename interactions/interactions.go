package reminteractions

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4/pgxpool"
)

func init() {
	functions.HTTP("interactions", interactions)
}

var pool *pgxpool.Pool

type Interaction struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  int    `json:"type"`
	Token string `json:"token"`
}

func createPool() (err error) {
	if pool == nil {
		ctx := context.Background()

		pool, err = pgxpool.Connect(ctx, os.Getenv("DATABASE_PRIVATE_URL"))
	}
	return
}

func interactions(writer http.ResponseWriter, request *http.Request) {

	writer.Header().Set("Access-Control-Allow-Origin", "https://discord.com")

	timestamp := request.Header.Get("X-Signature-Timestamp")

	publicKey, err := hex.DecodeString(os.Getenv("DISCORD_PUBLIC_KEY"))
	if err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid request signature")
		return
	}

	signature, err := hex.DecodeString(request.Header.Get("X-Signature-Ed25519"))
	if err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid request signature")
		return
	}

	rawBody, err := io.ReadAll(request.Body)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to parse request body: ", err)
		return
	}

	if !verifySignature(publicKey, rawBody, signature, timestamp) {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid request signature")
		return
	}

	var interaction Interaction

	if err := json.Unmarshal(rawBody, &interaction); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}

	if interaction.Type == 1 {
		writer.WriteHeader(http.StatusOK)
		fmt.Fprint(writer, `{"type":1}`)
		return
	}

	if interaction.Type == 2 {
		writer.WriteHeader(http.StatusOK)
		fmt.Println(string(rawBody))
		fmt.Fprint(writer, `{"type":5}`)
		client := &http.Client{
			Timeout: 1 * time.Nanosecond,
		}
		client.Post(os.Getenv("GCP_BASE_URI")+interaction.Name, "application/json", bytes.NewReader(rawBody))
		return
	}

	err = createPool()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Error creating pool: ", err)
		return
	}

}

func verifySignature(publicKey []byte, rawBody []byte, signature []byte, timestamp string) bool {
	body := string(rawBody)

	return ed25519.Verify(publicKey, []byte(timestamp+body), signature)

}
