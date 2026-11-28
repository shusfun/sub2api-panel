package panelupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	installRoot     = "/opt/sub2api-panel"
	stateRoot       = "/var/lib/sub2api-panel-updater"
	backupRoot      = "/var/lib/sub2api-panel-backup"
	maintenanceRoot = "/var/lib/sub2api-panel-maintenance"
	startPermit     = "/run/sub2api-panel-updater/start-allowed"
	serveUnit       = "sub2api-panel-serve.service"
	issuerUnit      = "sub2api-panel-issuer.service"
)

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

type systemdHost struct {
	serveAddress  string
	issuerAddress string
}

func RunExecutor() error {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return errors.New("unsupported updater architecture")
	}
	channel := os.Getenv("PANEL_UPDATE_CHANNEL")
	if channel == "" {
		channel = "stable"
	}
	if channel != "stable" && channel != "rc" {
		return errors.New("invalid PANEL_UPDATE_CHANNEL")
	}
	host := systemdHost{os.Getenv("PANEL_ADDR"), os.Getenv("PANEL_ISSUER_ADDR")}
	for _, addr := range []string{host.serveAddress, host.issuerAddress} {
		ip, port, err := net.SplitHostPort(addr)
		if err != nil || !net.ParseIP(ip).IsLoopback() || port == "" || port == "0" {
			return errors.New("updater requires fixed loopback addresses")
		}
	}
	for _, path := range []string{stateRoot, filepath.Join(installRoot, "releases")} {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(maintenanceRoot, 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(stateRoot, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	if os.Getenv("LISTEN_PID") != fmt.Sprint(os.Getpid()) || os.Getenv("LISTEN_FDS") != "1" {
		return errors.New("updater requires its systemd Unix socket")
	}
	socket := os.NewFile(3, "systemd updater socket")
	if socket == nil {
		return errors.New("systemd socket unavailable")
	}
	listener, err := net.FileListener(socket)
	socket.Close()
	if err != nil {
		return err
	}
	defer listener.Close()
	if _, ok := listener.(*net.UnixListener); !ok {
		return errors.New("updater socket must be Unix")
	}
	r, err := newRunner(stateRoot, channel, githubSource(), host)
	if err != nil {
		return err
	}
	return r.serve(listener)
}

func runCommand(ctx context.Context, path string, args ...string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func (systemdHost) current(_ context.Context) (string, string, error) {
	target, err := os.Readlink(filepath.Join(installRoot, "current"))
	if err != nil {
		return "", "", err
	}
	version, ok := strings.CutPrefix(target, "releases/")
	if !ok || !ValidVersion(version) || strings.ContainsAny(version, "/\\") {
		return "", "", errors.New("current link does not point to a version directory")
	}
	return version, version, nil
}

func (systemdHost) prepare(_ context.Context, version, source string) error {
	releases := filepath.Join(installRoot, "releases")
	staged, err := os.MkdirTemp(releases, ".release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	if err := stageFile(source, staged, binaryName, 0755); err != nil {
		return err
	}
	if err := stageFile(source, staged, grantsName, 0644); err != nil {
		return err
	}
	if err := os.Chmod(staged, 0755); err != nil {
		return err
	}
	if err := syncDir(staged); err != nil {
		return err
	}
	destination := filepath.Join(releases, version)
	if _, err := os.Lstat(destination); err == nil {
		// An earlier attempt can leave this version staged. Replace it only if the contents match.
		return errors.Join(
			identicalReleaseFile(filepath.Join(staged, binaryName), filepath.Join(destination, binaryName)),
			identicalReleaseFile(filepath.Join(staged, grantsName), filepath.Join(destination, grantsName)),
		)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(staged, destination); err != nil {
		return err
	}
	return syncDir(releases)
}

func stageFile(source, destination, name string, mode os.FileMode) error {
	in, err := os.Open(filepath.Join(source, name))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(filepath.Join(destination, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Sync(), out.Close())
}

func identicalReleaseFile(left, right string) error {
	a, err := os.ReadFile(left)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(right)
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return errors.New("staged release differs from verified file")
	}
	return nil
}

func replaceLink(name, version string) error {
	if !ValidVersion(version) || strings.ContainsAny(version, "/\\") {
		return errors.New("invalid release link version")
	}
	if info, err := os.Stat(filepath.Join(installRoot, "releases", version, "sub2api-panel")); err != nil || !info.Mode().IsRegular() {
		return errors.New("release binary is unavailable")
	}
	staged, err := os.MkdirTemp(installRoot, ".link-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	link := filepath.Join(staged, name)
	if err := os.Symlink(filepath.Join("releases", version), link); err != nil {
		return err
	}
	if err := os.Rename(link, filepath.Join(installRoot, name)); err != nil {
		return err
	}
	return syncDir(installRoot)
}

func touch(path string, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, mode)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close(), syncDir(filepath.Dir(path)))
}

func removeMarker(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func (systemdHost) setMaintenance(active bool) error {
	marker := filepath.Join(maintenanceRoot, "active")
	if active {
		if err := touch(marker, 0644); err != nil {
			return err
		}
		return removeMarker(startPermit)
	}
	if err := removeMarker(marker); err != nil {
		return err
	}
	return removeMarker(startPermit)
}

func unitCommand(ctx context.Context, budget time.Duration, action, unit string) error {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	return runCommand(ctx, "/usr/bin/systemctl", "--no-ask-password", action, unit)
}

func (systemdHost) stop(ctx context.Context) error {
	if err := unitCommand(ctx, 105*time.Second, "stop", serveUnit); err != nil {
		return err
	}
	return unitCommand(ctx, 105*time.Second, "stop", issuerUnit)
}

func (systemdHost) backup(ctx context.Context, id string) error {
	if err := unitCommand(ctx, 16*time.Minute, "start", "sub2api-panel-backup.service"); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(backupRoot, "pending.dump"), filepath.Join(backupRoot, id+".dump")); err != nil {
		return err
	}
	return syncDir(backupRoot)
}

func (systemdHost) migrate(ctx context.Context, version string) error {
	if err := replaceLink("pending", version); err != nil {
		return err
	}
	return unitCommand(ctx, 16*time.Minute, "start", "sub2api-panel-migrate.service")
}

func (systemdHost) selectBinary(version string) error { return replaceLink("current", version) }

func (h systemdHost) start(ctx context.Context, version string) error {
	if err := touch(startPermit, 0644); err != nil {
		return err
	}
	if err := unitCommand(ctx, time.Minute, "start", issuerUnit); err != nil {
		return err
	}
	if err := unitCommand(ctx, time.Minute, "start", serveUnit); err != nil {
		return err
	}
	return h.ready(ctx, version)
}

type readyReply struct {
	Version string `json:"version"`
	Service string `json:"service"`
}

func (h systemdHost) ready(ctx context.Context, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{}}
	defer client.CloseIdleConnections()
	for {
		if checkReady(ctx, client, h.issuerAddress, "issuer", version) &&
			checkReady(ctx, client, h.serveAddress, "serve", version) {
			return nil
		}
		if version == "0.1.0-rc.3" &&
			checkRC3(ctx, client, issuerUnit, h.issuerAddress, "/internal/grant", http.StatusForbidden) &&
			checkRC3(ctx, client, serveUnit, h.serveAddress, "/", http.StatusOK) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("issuer and serve did not become ready on version %s: %w", version, ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

// The already published RC.3 predates the readiness route. Its rollback can
// be identified by the unit's actual executable and its normal HTTP entry points.
func checkRC3(ctx context.Context, client *http.Client, unit, address, path string, status int) bool {
	out, err := exec.CommandContext(ctx, "/usr/bin/systemctl", "show", "--property=MainPID", "--value", unit).Output()
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || parseErr != nil || pid <= 1 {
		return false
	}
	process, err := os.Stat(fmt.Sprintf("/proc/%d/exe", pid))
	expected, targetErr := os.Stat(filepath.Join(installRoot, "releases", "0.1.0-rc.3", "sub2api-panel"))
	if err != nil || targetErr != nil || !os.SameFile(process, expected) {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+path, nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == status
}

func checkReady(ctx context.Context, client *http.Client, address, service, version string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/internal/panel-ready", nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	var result readyReply
	return res.StatusCode == http.StatusOK && readStrict(res.Body, 1024, &result) == nil &&
		result.Version == version && result.Service == service
}
