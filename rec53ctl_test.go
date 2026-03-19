package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRec53ctlInstallWritesManagedResourcesAndExplicitLogPath(t *testing.T) {
	workDir, fakeBin, systemctlLog := setupRec53ctlWorkspace(t)

	installDir := filepath.Join(workDir, "install")
	configDir := filepath.Join(workDir, "etc", "rec53")
	unitDir := filepath.Join(workDir, "systemd")
	logFile := filepath.Join(workDir, "var", "log", "rec53", "rec53.log")
	buildOutput := filepath.Join(workDir, "dist", "rec53")

	output, err := runRec53ctl(t, workDir, fakeBin, systemctlLog, map[string]string{
		"INSTALL_DIR":      installDir,
		"CONFIG_DIR":       configDir,
		"SYSTEMD_UNIT_DIR": unitDir,
		"LOG_FILE":         logFile,
		"BUILD_OUTPUT":     buildOutput,
	}, "install")
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, output)
	}

	unitFile := filepath.Join(unitDir, "rec53.service")
	unitData, err := os.ReadFile(unitFile)
	if err != nil {
		t.Fatalf("read unit file: %v", err)
	}
	unitText := string(unitData)
	if !strings.Contains(unitText, "# Managed by rec53ctl") {
		t.Fatalf("unit file missing management marker:\n%s", unitText)
	}
	if !strings.Contains(unitText, "-rec53.log "+logFile) {
		t.Fatalf("unit file missing explicit log path:\n%s", unitText)
	}

	markerData, err := os.ReadFile(filepath.Join(configDir, ".managed-by-rec53ctl"))
	if err != nil {
		t.Fatalf("read managed marker: %v", err)
	}
	if !strings.Contains(string(markerData), "managed-by=rec53ctl") {
		t.Fatalf("managed marker missing signature:\n%s", string(markerData))
	}

	if _, err := os.Stat(filepath.Dir(logFile)); err != nil {
		t.Fatalf("log directory was not created: %v", err)
	}

	systemctlData, err := os.ReadFile(systemctlLog)
	if err != nil {
		t.Fatalf("read systemctl log: %v", err)
	}
	if !strings.Contains(string(systemctlData), "enable --now rec53") {
		t.Fatalf("install did not enable service:\n%s", string(systemctlData))
	}
}

func TestRec53ctlUninstallPreservesConfigAndLogsByDefault(t *testing.T) {
	workDir, fakeBin, systemctlLog := setupRec53ctlWorkspace(t)

	installDir := filepath.Join(workDir, "install")
	configDir := filepath.Join(workDir, "etc", "rec53")
	unitDir := filepath.Join(workDir, "systemd")
	logFile := filepath.Join(workDir, "var", "log", "rec53", "rec53.log")

	writeManagedInstallFixture(t, installDir, configDir, unitDir, logFile)

	output, err := runRec53ctl(t, workDir, fakeBin, systemctlLog, map[string]string{
		"INSTALL_DIR":      installDir,
		"CONFIG_DIR":       configDir,
		"SYSTEMD_UNIT_DIR": unitDir,
		"LOG_FILE":         logFile,
	}, "uninstall")
	if err != nil {
		t.Fatalf("uninstall failed: %v\n%s", err, output)
	}

	assertNotExists(t, filepath.Join(installDir, "rec53"))
	assertNotExists(t, filepath.Join(unitDir, "rec53.service"))
	assertExists(t, filepath.Join(configDir, "config.yaml"))
	assertExists(t, filepath.Join(configDir, ".managed-by-rec53ctl"))
	assertExists(t, logFile)

	if !strings.Contains(output, "Preserved config and logs") {
		t.Fatalf("expected preservation message, got:\n%s", output)
	}
}

func TestRec53ctlUninstallPurgeRemovesManagedConfigAndLogs(t *testing.T) {
	workDir, fakeBin, systemctlLog := setupRec53ctlWorkspace(t)

	installDir := filepath.Join(workDir, "install")
	configDir := filepath.Join(workDir, "etc", "rec53")
	unitDir := filepath.Join(workDir, "systemd")
	logFile := filepath.Join(workDir, "var", "log", "rec53", "rec53.log")

	writeManagedInstallFixture(t, installDir, configDir, unitDir, logFile)

	output, err := runRec53ctl(t, workDir, fakeBin, systemctlLog, map[string]string{
		"INSTALL_DIR":      installDir,
		"CONFIG_DIR":       configDir,
		"SYSTEMD_UNIT_DIR": unitDir,
		"LOG_FILE":         logFile,
	}, "uninstall", "--purge")
	if err != nil {
		t.Fatalf("uninstall --purge failed: %v\n%s", err, output)
	}

	assertNotExists(t, filepath.Join(installDir, "rec53"))
	assertNotExists(t, filepath.Join(unitDir, "rec53.service"))
	assertNotExists(t, filepath.Join(configDir, "config.yaml"))
	assertNotExists(t, filepath.Join(configDir, ".managed-by-rec53ctl"))
	assertNotExists(t, logFile)
}

func TestRec53ctlInstallRefusesUnmanagedExistingUnit(t *testing.T) {
	workDir, fakeBin, systemctlLog := setupRec53ctlWorkspace(t)

	installDir := filepath.Join(workDir, "install")
	configDir := filepath.Join(workDir, "etc", "rec53")
	unitDir := filepath.Join(workDir, "systemd")
	logFile := filepath.Join(workDir, "var", "log", "rec53", "rec53.log")
	buildOutput := filepath.Join(workDir, "dist", "rec53")

	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		t.Fatalf("mkdir unit dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unitDir, "rec53.service"), []byte("[Unit]\nDescription=foreign service\n"), 0o644); err != nil {
		t.Fatalf("write unmanaged unit: %v", err)
	}

	output, err := runRec53ctl(t, workDir, fakeBin, systemctlLog, map[string]string{
		"INSTALL_DIR":      installDir,
		"CONFIG_DIR":       configDir,
		"SYSTEMD_UNIT_DIR": unitDir,
		"LOG_FILE":         logFile,
		"BUILD_OUTPUT":     buildOutput,
	}, "install")
	if err == nil {
		t.Fatalf("expected install to fail with unmanaged unit:\n%s", output)
	}
	if !strings.Contains(output, "already exists and is not managed by rec53ctl") {
		t.Fatalf("unexpected failure output:\n%s", output)
	}
}

func setupRec53ctlWorkspace(t *testing.T) (string, string, string) {
	t.Helper()

	workDir := t.TempDir()
	scriptData, err := os.ReadFile(filepath.Join(".", "rec53ctl"))
	if err != nil {
		t.Fatalf("read rec53ctl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "rec53ctl"), scriptData, 0o755); err != nil {
		t.Fatalf("write rec53ctl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "config.yaml"), []byte("dns:\n  listen: \"127.0.0.1:5353\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	fakeBin := filepath.Join(workDir, "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	systemctlLog := filepath.Join(workDir, "systemctl.log")

	writeExecutable(t, filepath.Join(fakeBin, "id"), "#!/bin/sh\nif [ \"$1\" = \"-u\" ]; then\n  echo 0\n  exit 0\nfi\n/usr/bin/id \"$@\"\n")
	writeExecutable(t, filepath.Join(fakeBin, "systemctl"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$SYSTEMCTL_LOG\"\nexit 0\n")
	writeExecutable(t, filepath.Join(fakeBin, "go"), "#!/bin/sh\nout=\"\"\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"-o\" ]; then\n    out=\"$2\"\n    shift 2\n    continue\n  fi\n  shift\ndone\nif [ -z \"$out\" ]; then\n  echo 'missing -o' >&2\n  exit 1\nfi\nmkdir -p \"$(dirname \"$out\")\"\nprintf '#!/bin/sh\\nexit 0\\n' > \"$out\"\nchmod 755 \"$out\"\n")

	return workDir, fakeBin, systemctlLog
}

func writeManagedInstallFixture(t *testing.T, installDir, configDir, unitDir, logFile string) {
	t.Helper()

	mustMkdirAll(t, installDir)
	mustMkdirAll(t, configDir)
	mustMkdirAll(t, unitDir)
	mustMkdirAll(t, filepath.Dir(logFile))

	if err := os.WriteFile(filepath.Join(installDir, "rec53"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("dns:\n  listen: \"127.0.0.1:5353\"\n"), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, ".managed-by-rec53ctl"), []byte("managed-by=rec53ctl\n"), 0o644); err != nil {
		t.Fatalf("write marker fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unitDir, "rec53.service"), []byte("# Managed by rec53ctl\n[Unit]\nDescription=rec53\n"), 0o644); err != nil {
		t.Fatalf("write unit fixture: %v", err)
	}
	if err := os.WriteFile(logFile, []byte("log"), 0o644); err != nil {
		t.Fatalf("write log fixture: %v", err)
	}
}

func runRec53ctl(t *testing.T, workDir, fakeBin, systemctlLog string, env map[string]string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command("/bin/bash", append([]string{"./rec53ctl"}, args...)...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"SYSTEMCTL_LOG="+systemctlLog,
	)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err=%v", path, err)
	}
}
