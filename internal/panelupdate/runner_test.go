package panelupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type rehearsalHost struct {
	calls   []string
	version string
	marker  bool
	failAt  string
}

func (h *rehearsalHost) record(stage string) error {
	h.calls = append(h.calls, stage)
	if stage == h.failAt {
		return errors.New("rehearsal failure")
	}
	return nil
}
func (h *rehearsalHost) current(context.Context) (string, string, error) {
	return h.version, h.version, nil
}
func (h *rehearsalHost) prepare(_ context.Context, v, dir string) error {
	info, err := os.Stat(filepath.Join(dir, binaryName))
	if err != nil {
		return err
	}
	if info.Size() != 6 {
		return errors.New("unexpected binary")
	}
	if _, err := os.Stat(filepath.Join(dir, grantsName)); err != nil {
		return err
	}
	return h.record("prepare " + v)
}
func (h *rehearsalHost) setMaintenance(active bool) error {
	h.marker = active
	return h.record(fmt.Sprintf("maintenance %v", active))
}
func (h *rehearsalHost) stop(context.Context) error           { return h.record("stop") }
func (h *rehearsalHost) backup(context.Context, string) error { return h.record("backup") }
func (h *rehearsalHost) migrate(_ context.Context, version string) error {
	return h.record("migrate " + version)
}
func (h *rehearsalHost) selectBinary(version string) error {
	h.version = version
	return h.record("switch " + version)
}
func (h *rehearsalHost) start(ctx context.Context, version string) error {
	if err := h.record("start " + version); err != nil {
		return err
	}
	return h.ready(ctx, version)
}
func (h *rehearsalHost) ready(_ context.Context, version string) error {
	if h.version != version {
		return errors.New("version is not selected")
	}
	return h.record("ready " + version)
}

func fakeSource(t *testing.T, manifests []Manifest, badChecksum bool) releaseSource {
	t.Helper()
	var archive bytes.Buffer
	compressed := gzip.NewWriter(&archive)
	tarball := tar.NewWriter(compressed)
	if err := tarball.WriteHeader(&tar.Header{Name: "sub2api-panel", Mode: 0755, Size: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarball.Write([]byte("binary")); err != nil {
		t.Fatal(err)
	}
	if err := tarball.WriteHeader(&tar.Header{Name: grantsName, Mode: 0644, Size: 9}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarball.Write([]byte("SELECT 1;")); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(tarball.Close(), compressed.Close()); err != nil {
		t.Fatal(err)
	}
	list := []release{}
	files := map[string][]byte{}
	for _, manifest := range manifests {
		product, ok := ProductForTag(manifest.Tag)
		if !ok {
			t.Fatal("bad fixture tag")
		}
		name := archiveName(product)
		sum := sha256.Sum256(archive.Bytes())
		if badChecksum {
			sum[0] ^= 1
		}
		body, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		items := map[string][]byte{
			manifestName:  body,
			checksumsName: fmt.Appendf(nil, "%x  %s\n", sum, name),
			name:          archive.Bytes(),
		}
		published := release{Tag: manifest.Tag, Notes: "演练发布说明", Immutable: true, Prerelease: rcPattern.MatchString(manifest.Tag)}
		for key, value := range items {
			published.Assets = append(published.Assets, asset{Name: key, Size: int64(len(value))})
			files[manifest.Tag+"/"+key] = value
		}
		list = append(list, published)
	}
	response, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	return releaseSource{
		request: func(_ context.Context, url string) (*http.Response, error) {
			if strings.HasPrefix(url, "https://api.github.com/") {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(response))}, nil
			}
			key := strings.TrimPrefix(url, "https://github.com/"+Repository+"/releases/download/")
			data, ok := files[key]
			if !ok {
				return &http.Response{StatusCode: 404, Body: http.NoBody}, nil
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}, nil
		},
		verify: func(context.Context, string, string) error { return nil },
	}
}

func TestReleaseChannelAndManifest(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	rc := Manifest{"0.1.1-rc.1", "v0.1.1-rc.1", []string{"0.1.0-rc.3"}, "none", false}
	source := fakeSource(t, []Manifest{rc}, false)
	if offer, err := source.find(ctx, dir, "0.1.0-rc.3", "stable", ""); err != nil || offer != nil {
		t.Fatalf("stable channel offered prerelease: %v, %v", offer, err)
	}
	if offer, err := source.find(ctx, dir, "0.1.0-rc.3", "rc", ""); err != nil || offer == nil || offer.info.Availability != "ready" {
		t.Fatalf("RC channel did not offer eligible prerelease: %v, %v", offer, err)
	}
	rc.Version = "0.1.1-rc.2"
	if _, err := fakeSource(t, []Manifest{rc}, false).find(ctx, dir, "0.1.0-rc.3", "rc", ""); err == nil {
		t.Fatal("mismatched manifest and tag were accepted")
	}
}

func TestSelectEligibleUpgrade(t *testing.T) {
	const current = "0.1.0-rc.3"
	first := Manifest{"0.1.0001", "v0.1.1", []string{current}, "none", true}
	second := Manifest{"0.1.0002", "v0.1.2", []string{first.Version}, "none", true}
	source := fakeSource(t, []Manifest{first, second}, false)
	dir := t.TempDir()
	next, err := source.find(t.Context(), dir, current, "stable", "")
	if err != nil || next == nil || next.info.Version != first.Version {
		t.Fatalf("selected upgrade = %v, %v", next, err)
	}
	next, err = source.find(t.Context(), dir, current, "stable", second.Version)
	if err != nil || next == nil || next.info.Availability != "unsupported_source" {
		t.Fatalf("explicit untested source should be blocked: %v, %v", next, err)
	}
}

func TestUpgradeRehearsal(t *testing.T) {
	dir := t.TempDir()
	host := &rehearsalHost{version: "0.1.0-rc.3"}
	for _, manifest := range []Manifest{
		{"0.1.0001", "v0.1.1", []string{"0.1.0-rc.3"}, "apply", true},
		{"0.1.0002", "v0.1.2", []string{"0.1.0001"}, "none", false},
	} {
		runner, err := newRunner(dir, "stable", fakeSource(t, []Manifest{manifest}, false), host)
		if err != nil {
			t.Fatal(err)
		}
		from := host.version
		req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(fmt.Sprintf("{\"version\":%q,\"actor\":1}", manifest.Version)))
		req.Header.Set("traceparent", "00-5874588eb8fa453d96963a1f2686dbb2-5874588eb8fa453d-01")
		response := httptest.NewRecorder()
		runner.submit(response, req)
		if response.Code != http.StatusAccepted {
			t.Fatalf("submit status %d", response.Code)
		}
		job := awaitJob(t, runner)
		if job.Phase != succeeded || job.From != from || host.version != manifest.Version || host.marker {
			t.Fatalf("upgrade result: %+v, host: %+v", job, host)
		}
		reloaded, err := newRunner(dir, "stable", releaseSource{}, host)
		if err != nil || reloaded.job == nil || reloaded.job.ID != job.ID || reloaded.job.Trace != "5874588eb8fa453d96963a1f2686dbb2" {
			t.Fatalf("stored result or trace missing after restart: %v", err)
		}
	}
	if !slices.Equal(host.calls, []string{
		"prepare 0.1.0001", "maintenance true", "stop", "backup", "migrate 0.1.0001", "switch 0.1.0001", "start 0.1.0001", "ready 0.1.0001", "maintenance false",
		"prepare 0.1.0002", "maintenance true", "stop", "backup", "switch 0.1.0002", "start 0.1.0002", "ready 0.1.0002", "maintenance false",
	}) {
		t.Fatalf("unexpected actions: %v", host.calls)
	}
}

func awaitJob(t *testing.T, r *runner) persistedJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		job := *r.job
		r.mu.Unlock()
		if !busy(&job) || job.Phase == intervention {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("update did not finish")
	return persistedJob{}
}

func TestUpdateFailureRecovery(t *testing.T) {
	const from = "0.1.0001"
	const to = "0.1.0002"
	for _, test := range []struct {
		name, failAt, want              string
		checksum, rollback, maintenance bool
	}{
		{"archive checksum", "", failed, true, false, false},
		{"migration uncertain", "migrate " + to, intervention, false, true, true},
		{"ready failed and rollback allowed", "ready " + to, rolledBack, false, true, false},
		{"ready failed and rollback forbidden", "ready " + to, intervention, false, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := &rehearsalHost{version: from, failAt: test.failAt}
			manifest := Manifest{to, "v0.1.2", []string{from}, "apply", test.rollback}
			r, err := newRunner(t.TempDir(), "stable", fakeSource(t, []Manifest{manifest}, test.checksum), h)
			if err != nil {
				t.Fatal(err)
			}
			job := persistedJob{Job: Job{ID: "dddddddddddddddddddddddddddddddd", From: from, To: to, Phase: queued}, Previous: from}
			r.run(job)
			if r.job.Phase != test.want || h.marker != test.maintenance {
				t.Fatalf("job=%+v actions=%v", r.job, h.calls)
			}
			if test.checksum && len(h.calls) != 0 {
				t.Fatalf("changed host using bad archive: %v", h.calls)
			}
			if test.want == rolledBack && h.version != from {
				t.Fatal("previous binary was not selected")
			}
		})
	}
}

func TestInterruptedMigrationRequiresIntervention(t *testing.T) {
	h := &rehearsalHost{version: "0.1.0001", marker: true}
	dir := t.TempDir()
	r, err := newRunner(dir, "stable", releaseSource{}, h)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.save(persistedJob{
		Job:      Job{ID: "dddddddddddddddddddddddddddddddd", From: h.version, To: "0.1.0002", Phase: migrate},
		Previous: h.version, BinaryRollback: true, Migration: "apply",
	}); err != nil {
		t.Fatal(err)
	}
	r, err = newRunner(dir, "stable", releaseSource{}, h)
	if err != nil {
		t.Fatal(err)
	}
	r.resume(*r.job)
	if r.job.Phase != intervention || !h.marker || !slices.Equal(h.calls, []string{"maintenance true", "stop"}) {
		t.Fatalf("interrupted migrate was not contained: %+v, %v", r.job, h.calls)
	}
	res := httptest.NewRecorder()
	r.submit(res, httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader("{\"version\":\"0.1.0002\",\"actor\":1}")))
	if res.Code != http.StatusConflict {
		t.Fatalf("concurrent request allowed: %d", res.Code)
	}
}

func TestResumeAfterMigrationDoesNotMigrateAgain(t *testing.T) {
	const from = "0.1.0001"
	const to = "0.1.0002"
	h := &rehearsalHost{version: from, marker: true}
	dir := t.TempDir()
	r, err := newRunner(dir, "stable", releaseSource{}, h)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.save(persistedJob{
		Job:      Job{ID: "dddddddddddddddddddddddddddddddd", From: from, To: to, Phase: switching},
		Previous: from, Proposed: to, Migration: "apply",
	}); err != nil {
		t.Fatal(err)
	}
	r, err = newRunner(dir, "stable", releaseSource{}, h)
	if err != nil {
		t.Fatal(err)
	}
	r.resume(*r.job)
	if r.job.Phase != succeeded || h.marker || h.version != to || !slices.Equal(h.calls,
		[]string{"maintenance true", "stop", "switch " + to, "start " + to, "ready " + to, "maintenance false"}) {
		t.Fatalf("post-migration activation did not recover: %+v, %v", r.job, h.calls)
	}
}
