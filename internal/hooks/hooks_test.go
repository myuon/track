package hooks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/myuon/track/internal/store/sqlite"
)

func TestRunEventExecutesHook(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	outFile := filepath.Join(tmp, "hook.out")
	cmd := "/bin/sh -c 'echo $TRACK_EVENT:$TRACK_ISSUE_ID >> " + outFile + "'"
	if err := store.AddHook(ctx, IssueCompleted, cmd, ""); err != nil {
		t.Fatalf("add hook: %v", err)
	}

	if err := RunEvent(ctx, store, IssueCompleted, "TRK-1"); err != nil {
		t.Fatalf("run event: %v", err)
	}

	raw, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read hook output: %v", err)
	}
	if !strings.Contains(string(raw), "issue.completed:TRK-1") {
		t.Fatalf("unexpected hook output: %q", string(raw))
	}
}

func TestAutoOrganizeOnCreatedScript(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "scripts", "hooks", "auto-organize-on-created.sh")
	nextActionDefault := "planning: refine spec and decide ready/user"

	t.Run("sets assignee and next_action when both are missing", func(t *testing.T) {
		stateDir := t.TempDir()
		seedFakeTrackIssue(t, stateDir, "TRK-1", "id: TRK-1\nstatus: todo\n")
		runAutoOrganizeScript(t, scriptPath, stateDir, "TRK-1")

		raw, err := os.ReadFile(filepath.Join(stateDir, "set.log"))
		if err != nil {
			t.Fatalf("read set log: %v", err)
		}
		got := strings.TrimSpace(string(raw))
		want := "TRK-1 --assignee agent --next-action " + nextActionDefault
		if got != want {
			t.Fatalf("set log mismatch:\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("does nothing when status is not todo", func(t *testing.T) {
		stateDir := t.TempDir()
		seedFakeTrackIssue(t, stateDir, "TRK-2", "id: TRK-2\nstatus: ready\n")
		runAutoOrganizeScript(t, scriptPath, stateDir, "TRK-2")
		assertSetLogNotExists(t, stateDir)
	})

	t.Run("does not overwrite existing assignee and next_action", func(t *testing.T) {
		stateDir := t.TempDir()
		seedFakeTrackIssue(t, stateDir, "TRK-3", "id: TRK-3\nstatus: todo\nassignee: user\nnext_action: keep me\n")
		runAutoOrganizeScript(t, scriptPath, stateDir, "TRK-3")
		assertSetLogNotExists(t, stateDir)
	})

	t.Run("sets next_action only when assignee already exists", func(t *testing.T) {
		stateDir := t.TempDir()
		seedFakeTrackIssue(t, stateDir, "TRK-4", "id: TRK-4\nstatus: todo\nassignee: user\n")
		runAutoOrganizeScript(t, scriptPath, stateDir, "TRK-4")

		raw, err := os.ReadFile(filepath.Join(stateDir, "set.log"))
		if err != nil {
			t.Fatalf("read set log: %v", err)
		}
		got := strings.TrimSpace(string(raw))
		want := "TRK-4 --next-action " + nextActionDefault
		if got != want {
			t.Fatalf("set log mismatch:\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("sets assignee only when next_action already exists", func(t *testing.T) {
		stateDir := t.TempDir()
		seedFakeTrackIssue(t, stateDir, "TRK-5", "id: TRK-5\nstatus: todo\nnext_action: keep me\n")
		runAutoOrganizeScript(t, scriptPath, stateDir, "TRK-5")

		raw, err := os.ReadFile(filepath.Join(stateDir, "set.log"))
		if err != nil {
			t.Fatalf("read set log: %v", err)
		}
		got := strings.TrimSpace(string(raw))
		want := "TRK-5 --assignee agent"
		if got != want {
			t.Fatalf("set log mismatch:\n got: %q\nwant: %q", got, want)
		}
	})
}

func runAutoOrganizeScript(t *testing.T, scriptPath, stateDir, issueID string) {
	t.Helper()

	fakeBinDir := filepath.Join(stateDir, "bin")
	if err := os.MkdirAll(fakeBinDir, 0o755); err != nil {
		t.Fatalf("mkdir fake bin dir: %v", err)
	}

	fakeTrack := filepath.Join(fakeBinDir, "track")
	fakeTrackBody := "#!/bin/sh\n" +
		"set -eu\n" +
		"STATE_DIR=\"${FAKE_TRACK_STATE_DIR:?}\"\n" +
		"cmd=\"${1:-}\"\n" +
		"case \"$cmd\" in\n" +
		"  show)\n" +
		"    issue=\"${2:?}\"\n" +
		"    cat \"$STATE_DIR/$issue.show\"\n" +
		"    ;;\n" +
		"  set)\n" +
		"    issue=\"${2:?}\"\n" +
		"    shift 2\n" +
		"    printf \"%s %s\\n\" \"$issue\" \"$*\" >> \"$STATE_DIR/set.log\"\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"unsupported fake track subcommand: $cmd\" >&2\n" +
		"    exit 91\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(fakeTrack, []byte(fakeTrackBody), 0o755); err != nil {
		t.Fatalf("write fake track: %v", err)
	}

	cmd := exec.Command(scriptPath)
	cmd.Env = append(os.Environ(),
		"TRACK_ISSUE_ID="+issueID,
		"FAKE_TRACK_STATE_DIR="+stateDir,
		"PATH="+fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run auto-organize script: %v, out=%s", err, string(out))
	}
}

func seedFakeTrackIssue(t *testing.T, stateDir, issueID, showOutput string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(stateDir, issueID+".show"), []byte(showOutput), 0o644); err != nil {
		t.Fatalf("seed fake issue: %v", err)
	}
}

func assertSetLogNotExists(t *testing.T, stateDir string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(stateDir, "set.log"))
	if err == nil {
		t.Fatalf("set.log should not be created")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat set.log: %v", err)
	}
}
