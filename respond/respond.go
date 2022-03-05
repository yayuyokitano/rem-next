package remrespond

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/yayuyokitano/kitaipu"
	"github.com/yayuyokitano/remsponder"
)

type VerifiedInteraction struct {
	Interaction kitaipu.Command `json:"interaction"`
	Token       string          `json:"token"`
}

func init() {
	functions.HTTP("respond", respond)
}

var pool *pgxpool.Pool

func createPool() (err error) {
	if pool == nil {
		ctx := context.Background()

		pool, err = pgxpool.Connect(ctx, os.Getenv("DATABASE_PRIVATE_URL"))
	}
	return
}

func respond(writer http.ResponseWriter, request *http.Request) {

	var verifiedInteraction VerifiedInteraction
	if err := json.NewDecoder(request.Body).Decode(&verifiedInteraction); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode interaction", err)
		return
	}

	if verifiedInteraction.Token != os.Getenv("DISCORD_SECRET") {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Invalid request token")
		return
	}
	interaction := verifiedInteraction.Interaction

	res, err := remsponder.CallInteraction(interaction)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
	}
	contentType, b, err := res.Prepare()
	fmt.Println(string(b))

	client := http.Client{}
	url := fmt.Sprintf("%s/webhooks/%s/%s/messages/@original", os.Getenv("DISCORD_BASE_URI"), interaction.ApplicationID, interaction.Token)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(b))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := client.Do(req)
	rawBody, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(rawBody))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
	}
	return

}
