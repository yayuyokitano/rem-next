package reminteractions

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/yayuyokitano/kitaipu"
)

var client *pubsub.Client

func init() {
	var err error

	functions.HTTP("interactions", interactions)

	client, err = pubsub.NewClient(context.Background(), os.Getenv("GCP_PROJECT_ID"))
	if err != nil {
		log.Fatalf("pubsub.NewClient: %v", err)
	}
}

type publishRequest struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

type VerifiedInteraction struct {
	Interaction kitaipu.Command `json:"interaction"`
	Token       string          `json:"token"`
}

var pool *pgxpool.Pool

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
		fmt.Print("Failed to parse request body: ", err)
		return
	}

	if !verifySignature(publicKey, rawBody, signature, timestamp) {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Print("Invalid request signature")
		return
	}

	var interaction kitaipu.Command

	if err := json.Unmarshal(rawBody, &interaction); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Print("Failed to decode request body", err)
		fmt.Print(string(rawBody))
		return
	}

	if interaction.Type == 1 {
		writer.WriteHeader(http.StatusOK)
		fmt.Fprint(writer, `{"type":1}`)
		return
	}

	if interaction.Type == 2 {

		verifiedInteraction := VerifiedInteraction{
			Interaction: interaction,
			Token:       os.Getenv("DISCORD_SECRET"),
		}

		jsonPayload, err := json.Marshal(verifiedInteraction)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			fmt.Print("Failed to encode request body", err)
			return
		}

		p := publishRequest{
			Topic:   "responder",
			Message: string(jsonPayload),
		}

		m := &pubsub.Message{
			Data: []byte(p.Message),
		}

		_, err = client.Topic(p.Topic).Publish(request.Context(), m).Get(request.Context())
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			fmt.Print("Failed to publish message", err)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Print(string(rawBody))
		fmt.Fprint(writer, `{"type":5}`)
		return

	}

	err = createPool()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Print("Error creating pool: ", err)
		return
	}

}

func verifySignature(publicKey []byte, rawBody []byte, signature []byte, timestamp string) bool {
	body := string(rawBody)

	return ed25519.Verify(publicKey, []byte(timestamp+body), signature)

}
