package remenvtest

import "testing"

func TestTestEnv(t *testing.T) {
	expected := "lcJ6FO-1hMegLVeL1MonRs6-W_2pTa8k"
	actual := testEnv("DISCORD_SECRET")
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}
