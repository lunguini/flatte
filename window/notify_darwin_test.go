package window

import (
	"context"
	"github.com/lunguini/flatte"
	"slices"
	"testing"
)

func TestNotificationCommandSeparatesArguments(t *testing.T) {
	cmd := notificationCommand(context.Background(), flatte.Notification{Title: "-title", Body: "-body"})
	want := []string{"--", "-title", "-body"}
	if !slices.Equal(cmd.Args[len(cmd.Args)-3:], want) {
		t.Fatalf("notification arguments: %q", cmd.Args)
	}
}
