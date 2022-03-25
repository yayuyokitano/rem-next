package remrolereward

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

func TestRoleReward(t *testing.T) {

	err := createPool()
	if err != nil {
		t.Errorf("Failed to create pool: %s\n", err)
	}

	var curPersistent bool
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

	sub := testClient.Subscription("remrakutest")
	cctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	token, err := strconv.ParseInt(os.Getenv("REM_TEST_TOKEN"), 10, 64)
	if err != nil {
		t.Errorf("Failed to parse token: %s\n", err)
		return
	}

	params := Params{
		GuildID:    os.Getenv("REM_TEST_GUILDID"),
		RoleID:     os.Getenv("REM_TEST_ROLEID"),
		UserID:     os.Getenv("REM_TEST_USERID"),
		Token:      token,
		Level:      100,
		Persistent: true,
		State:      true,
	}
	jsonParams, err := json.Marshal(params)
	if err != nil {
		t.Errorf("Failed to marshal params: %s\n", err)
		return
	}

	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/rolereward", bytes.NewReader(jsonParams))

	roleReward(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
		return
	}

	time.Sleep(2 * time.Second)

	err = sub.Receive(cctx, func(_ context.Context, msg *pubsub.Message) {
		receivedMessage = true
		curPersistent, curState, curErr = handlePubsub(msg, t)
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

	if curPersistent != true {
		t.Errorf("Expected true, got %t\n", curPersistent)
		return
	}

	if curState != true {
		t.Errorf("Expected true, got %t\n", curState)
		return
	}

	curPersistent, curErr = checkSQL()
	if curErr != nil {
		t.Errorf("Failed to check SQL: %s\n", curErr)
		return
	}

	if curPersistent != true {
		t.Errorf("Expected true, got %t\n", curPersistent)
		return
	}

	cctx, cancel = context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	params.Persistent = false
	jsonParams, err = json.Marshal(params)
	if err != nil {
		t.Errorf("Failed to marshal params: %s\n", err)
		return
	}

	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/rolereward", bytes.NewReader(jsonParams))

	roleReward(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
		return
	}

	err = sub.Receive(cctx, func(_ context.Context, msg *pubsub.Message) {
		receivedMessage = true
		curPersistent, curState, curErr = handlePubsub(msg, t)
	})
	if err != nil {
		t.Errorf("Receive: %v", err)
		return
	}

	time.Sleep(2 * time.Second)

	if curErr != nil || !receivedMessage {
		t.Errorf("Failed to receive message: %s\n", curErr)
		return
	}

	receivedMessage = false

	if curPersistent != false {
		t.Errorf("Expected false, got %t\n", curPersistent)
		return
	}

	if curState != true {
		t.Errorf("Expected true, got %t\n", curState)
		return
	}

	curPersistent, curErr = checkSQL()
	if curErr != nil {
		t.Errorf("Failed to check SQL: %s\n", curErr)
		return
	}

	if curPersistent != false {
		t.Errorf("Expected false, got %t\n", curPersistent)
		return
	}

	params.State = false

	cctx, cancel = context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	jsonParams, err = json.Marshal(params)
	if err != nil {
		t.Errorf("Failed to marshal params: %s\n", err)
		return
	}

	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/rolereward", bytes.NewReader(jsonParams))

	roleReward(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
		return
	}

	err = sub.Receive(cctx, func(_ context.Context, msg *pubsub.Message) {
		receivedMessage = true
		curPersistent, curState, curErr = handlePubsub(msg, t)
	})
	if err != nil {
		t.Errorf("Receive: %v", err)
		return
	}

	time.Sleep(2 * time.Second)

	if curErr != nil || !receivedMessage {
		t.Errorf("Failed to receive message: %s\n", curErr)
		return
	}

	receivedMessage = false

	if curState != false {
		t.Errorf("Expected false, got %t\n", curState)
		return
	}

	curPersistent, curErr = checkSQL()
	if curErr.Error() != "no rows in result set" {
		t.Errorf("Incorrect error: expected %s, got %s\n", "no rows in result set", curErr)
		return
	}

}

func handlePubsub(msg *pubsub.Message, t *testing.T) (persistent bool, state bool, err error) {
	msg.Ack()
	var message rolerewardMessage
	err = json.Unmarshal(msg.Data, &message)
	if err != nil {
		err = errors.New(fmt.Sprintf("json.Unmarshal: %v", err))
		return
	}

	if message.Type != "rolereward" || message.GuildID != os.Getenv("REM_TEST_GUILDID") || message.RoleID != os.Getenv("REM_TEST_ROLEID") {
		err = errors.New(fmt.Sprintf("Invalid message: %v", message))
		return
	}

	persistent = message.Persistent
	state = message.State
	return
}

func checkSQL() (r bool, err error) {

	row := pool.QueryRow(context.Background(), "SELECT persistent FROM rolerewards WHERE guildid = $1", os.Getenv("REM_TEST_GUILDID"))
	err = row.Scan(&r)

	return

}
