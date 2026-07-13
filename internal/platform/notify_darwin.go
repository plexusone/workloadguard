//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Notify sends a macOS notification using osascript.
func Notify(ctx context.Context, title, message string) error {
	script := fmt.Sprintf(
		`display notification %s with title %s`,
		escapeAppleScript(message),
		escapeAppleScript(title),
	)

	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
	return cmd.Run()
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
