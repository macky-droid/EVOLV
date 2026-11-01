package agent

import (
	"strings"
	"testing"
)

func TestCheckShellCommand_DestructiveBlocked(t *testing.T) {
	destructiveCommands := []struct {
		cmd    string
		reason string
	}{
		{"rm -rf /", "root"},
		{"rm -rf /*", "root"},
		{"rm -fr ~", "home"},
		{"rm -rf $HOME", "home"},
		{"rm -rf /etc/hosts", "system"},
		{"rm -rf /var/log", "system"},
		{"rd /s /q C:\\", "drive"},
		{"rmdir /s /q C:/", "drive"},
		{"del /s /q C:\\windows\\system32", "windows"},
		{"format C:", "formatting"},
		{"mkfs.ext4 /dev/sda1", "filesystem"},
		{"dd if=/dev/zero of=/dev/sda", "raw block"},
		{":(){ :|:& };:", "fork bomb"},
		{"shutdown -h now", "shutdown"},
		{"reboot", "reboot"},
		{"init 0", "runlevel"},
		{"echo dangerous && rm -rf /", "root"},
	}

	for _, tc := range destructiveCommands {
		t.Run(tc.cmd, func(t *testing.T) {
			decision := CheckShellCommand(tc.cmd, "", false)
			if !decision.Blocked {
				t.Fatalf("expected command %q to be blocked", tc.cmd)
			}
			if !decision.RequiresConfirm {
				t.Fatalf("expected RequiresConfirm to be true for %q", tc.cmd)
			}
			if !strings.Contains(strings.ToLower(decision.Reason), tc.reason) {
				t.Fatalf("expected reason to contain %q, got: %s", tc.reason, decision.Reason)
			}
		})
	}
}

func TestCheckShellCommand_ConfirmedAllowed(t *testing.T) {
	decision := CheckShellCommand("rm -rf /", "", true)
	if decision.Blocked {
		t.Fatalf("expected confirmed dangerous command to be allowed, got blocked: %s", decision.Reason)
	}
}

func TestCheckShellCommand_SafeCommandsAllowed(t *testing.T) {
	safeCommands := []string{
		"echo hello world",
		"rm -rf node_modules",
		"rm -rf ./build",
		"rm -rf dist",
		"del build.log",
		"go test -v ./...",
		"npm run build",
		"git status",
		"dir",
		"ls -la /tmp",
		"cat README.md",
	}

	for _, cmd := range safeCommands {
		t.Run(cmd, func(t *testing.T) {
			decision := CheckShellCommand(cmd, "", false)
			if decision.Blocked {
				t.Fatalf("safe command %q was unexpectedly blocked: %s", cmd, decision.Reason)
			}
		})
	}
}

func TestAgentShellExecutionGuard(t *testing.T) {
	a := testAgent(t)

	// Attempt destructive command without confirm
	res := execOK(t, a, action{
		Action:  "shell",
		Command: "rm -rf /",
	})

	if res["ok"] != false {
		t.Fatal("expected unconfirmed destructive shell to fail")
	}
	if !strings.Contains(res["error"].(string), "blocked destructive command") {
		t.Fatalf("unexpected error message: %v", res["error"])
	}
	if res["requires_confirmation"] != true {
		t.Fatalf("expected requires_confirmation true, got %v", res["requires_confirmation"])
	}
}
