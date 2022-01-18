package remenvtest

import "os"

func testEnv(name string) string {
	return os.Getenv(name)
}
