package remmodifylevels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DBEntry []interface{}

type DBEntries [][]interface{}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Xp       int64  `json:"xp"`
}

type RoleReward struct {
	Level int `json:"rank"`
	Role  struct {
		ID    string `json:"id"`
		Color int    `json:"color"`
	} `json:"role"`
}

type Mee6 struct {
	Page        int          `json:"page"`
	Users       []User       `json:"players"`
	RoleRewards []RoleReward `json:"role_rewards"`
}

func init() {
	functions.HTTP("modify-levels", modifyLevels)
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

var pool *pgxpool.Pool

func createPool() (err error) {
	if pool == nil {
		ctx := context.Background()

		pool, err = pgxpool.Connect(ctx, os.Getenv("DATABASE_PRIVATE_URL"))
	}
	return
}

type LevelParams struct {
	Operation string `json:"operation"`
	GuildID   string `json:"guildID"`
	UserID    string `json:"userID"`
	CallerID  string `json:"callerID"`
	Token     int64  `json:"token"`
	Source    string `json:"source"`
}

func modifyLevels(writer http.ResponseWriter, request *http.Request) {

	corsHandler(writer, request)

	err := createPool()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to create pool: ", err)
		return
	}

	var params LevelParams

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to decode request body", err)
		return
	}

	fmt.Println(params)

	err = confirmPermission(params.GuildID, params.CallerID, params.Token)
	if err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, "Failed to confirm permission: ", err)
		return
	}

	fmt.Println(params)
	fmt.Println(params.Operation)

	switch params.Operation {
	case "reset":
		err = resetLevels(params.GuildID)
		break
	case "import":
		err = importLevels(params.GuildID, params.Source)
		break
	}
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(writer, "Failed to modify levels: ", err)
		return
	}
	fmt.Fprint(writer, "Completed successfully")

}

func confirmPermission(guildID string, callerID string, token int64) (err error) {

	client := &http.Client{}
	resp, err := client.Post(os.Getenv("GCP_BASE_URI")+"/confirm-permission", "application/json", strings.NewReader(fmt.Sprintf(`{"guildID":"%s","userID":"%s","token":%d}`, guildID, callerID, token)))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = errors.New("Invalid token or user, or insufficient guild permissions: " + string(rawBody))
		return
	}
	return
}

func resetLevels(guildID string) (err error) {
	_, err = pool.Exec(context.Background(), "DELETE FROM guildxp WHERE guildID = $1", guildID)
	_, err = pool.Exec(context.Background(), "DELETE FROM rolerewards WHERE guildID = $1", guildID)
	return
}

func importLevels(guildID string, source string) (err error) {
	err = resetLevels(guildID)
	if err != nil {
		return
	}

	switch source {
	case "MEE6":
		err = importMEE6(guildID)
		break
	default:
		err = errors.New("Invalid source")
		break
	}
	return

}

func handleMee6Page(guildID string, curPage int, m *Mee6) (lastPage bool, err error) {
	fmt.Println("Handling page", curPage)
	resp, err := http.Get(fmt.Sprintf("https://mee6.xyz/api/plugins/levels/leaderboard/%s?page=%d", guildID, curPage))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New("Failed to get page")
		return
	}
	err = json.NewDecoder(resp.Body).Decode(m)
	if err != nil {
		return
	}
	if len(m.Users) == 0 {
		lastPage = true
	}
	fmt.Println(m)
	return
}

func importMEE6(guildID string) (err error) {
	lastPage := false
	curPage := 0
	users := make(DBEntries, 0)
	roleRewards := make([]RoleReward, 0)
	for !lastPage {
		var m Mee6
		lastPage, err = handleMee6Page(guildID, curPage, &m)
		if err != nil {
			lastPage, err = handleMee6Page(guildID, curPage, &m)
			if err != nil {
				return
			}
		}
		for _, user := range m.Users {
			fmt.Println(user)
			users = append(users, DBEntry{
				guildID,
				user.ID,
				user.Username,
				user.Avatar,
				user.Xp,
			})
		}
		if curPage == 0 {
			roleRewards = m.RoleRewards
		}
		curPage++
	}
	fmt.Println(users)
	_, err = pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"guildxp"},
		[]string{"guildid", "userid", "nickname", "avatar", "xp"},
		pgx.CopyFromRows(users),
	)
	if err != nil {
		return
	}

	roleRewardInsert := make(DBEntries, 0)
	for _, r := range roleRewards {
		roleRewardInsert = append(roleRewardInsert, DBEntry{
			guildID,
			r.Role.ID,
			r.Level,
			r.Role.Color,
		})
	}
	_, err = pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"rolerewards"},
		[]string{"guildid", "roleid", "level", "color"},
		pgx.CopyFromRows(roleRewardInsert),
	)

	return

}
