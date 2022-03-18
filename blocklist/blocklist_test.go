package remblocklist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
)

func TestGuilds(t *testing.T) {

	err := createPool()
	if err != nil {
		t.Errorf("Failed to create pool: %s\n", err)
	}

	var curState bool
	var curErr error
	receivedMessage := false
	ctx := context.Background()
	testClient, err := pubsub.NewClient(ctx, os.Getenv("GCP_PROJECT_ID"))
	if err != nil {
		t.Errorf("pubsub.NewClient: %v", err)
		return
	}
	defer testClient.Close()

	sub := testClient.Subscription("remrakusub")
	cctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	token, err := strconv.ParseInt(os.Getenv("REM_TEST_TOKEN"), 10, 64)
	if err != nil {
		t.Errorf("Failed to parse token: %s\n", err)
		return
	}

	params := Params{
		GuildID:   os.Getenv("REM_TEST_GUILDID"),
		ChannelID: os.Getenv("REM_TEST_CHANNELID"),
		UserID:    os.Getenv("REM_TEST_USERID"),
		Token:     token,
		ListType:  "xpgain",
		State:     true,
	}
	jsonParams, err := json.Marshal(params)
	if err != nil {
		t.Errorf("Failed to marshal params: %s\n", err)
		return
	}

	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/blocklist", bytes.NewReader(jsonParams))

	blocklist(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
		return
	}

	time.Sleep(2 * time.Second)

	err = sub.Receive(cctx, func(_ context.Context, msg *pubsub.Message) {
		receivedMessage = true
		curState, curErr = handlePubsub(msg, t)
	})
	if err != nil {
		t.Errorf("Receive: %v", err)
		return
	}

	if curErr != nil || !receivedMessage {
		t.Errorf("Failed to receive message: %s\n", curErr)
		return
	}

	receivedMessage = false

	if curState != true {
		t.Errorf("Expected true, got %t\n", curState)
		return
	}

	curState, curErr = checkSQL()
	if curErr != nil {
		t.Errorf("Failed to check SQL: %s\n", curErr)
		return
	}

	if curState != true {
		t.Errorf("Expected true, got %t\n", curState)
		return
	}

	cctx, cancel = context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	params.State = false
	jsonParams, err = json.Marshal(params)
	if err != nil {
		t.Errorf("Failed to marshal params: %s\n", err)
		return
	}

	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/blocklist", bytes.NewReader(jsonParams))

	blocklist(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
		return
	}

	err = sub.Receive(cctx, func(_ context.Context, msg *pubsub.Message) {
		receivedMessage = true
		curState, curErr = handlePubsub(msg, t)
	})
	if err != nil {
		t.Errorf("Receive: %v", err)
		return
	}

	time.Sleep(10 * time.Second)

	if curErr != nil || !receivedMessage {
		t.Errorf("Failed to receive message: %s\n", curErr)
		return
	}

	receivedMessage = false

	if curState != false {
		t.Errorf("Expected false, got %t\n", curState)
		return
	}

	curState, curErr = checkSQL()
	if curErr != nil {
		t.Errorf("Failed to check SQL: %s\n", curErr)
		return
	}

	if curState != false {
		t.Errorf("Expected false, got %t\n", curState)
		return
	}

	params.ListType = "xpgai"
	jsonParams, err = json.Marshal(params)
	if err != nil {
		t.Errorf("Failed to marshal params: %s\n", err)
		return
	}

	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/blocklist", bytes.NewReader(jsonParams))

	blocklist(writer, request)

	time.Sleep(5 * time.Second)

	if writer.Code != http.StatusInternalServerError {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusInternalServerError, writer.Code, writer.Body)
		return
	}

}

func handlePubsub(msg *pubsub.Message, t *testing.T) (state bool, err error) {
	msg.Ack()
	var message blocklistMessage
	err = json.Unmarshal(msg.Data, &message)
	if err != nil {
		err = errors.New(fmt.Sprintf("json.Unmarshal: %v", err))
		return
	}

	if message.ListType != "xpgain" || message.GuildID != os.Getenv("REM_TEST_GUILDID") || message.ChannelID != os.Getenv("REM_TEST_CHANNELID") {
		err = errors.New(fmt.Sprintf("Invalid message: %v", message))
		return
	}

	state = message.State
	return
}

func checkSQL() (r bool, err error) {

	row := pool.QueryRow(context.Background(), "SELECT xpgain FROM channelblocklist WHERE channelID = $1", os.Getenv("REM_TEST_CHANNELID"))
	err = row.Scan(&r)

	return

}
