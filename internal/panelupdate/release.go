package panelupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
)

const (
	Repository    = "shusfun/sub2api-panel"
	manifestName  = "update-manifest.json"
	checksumsName = "checksums.txt"
	releasePage   = "https://github.com/" + Repository + "/releases/tag/"
	maxManifest   = 32 << 10
)

type Manifest struct {
	Version        string   `json:"version"`
	Tag            string   `json:"tag"`
	UpgradeFrom    []string `json:"upgrade_from"`
	Migration      string   `json:"migration"`
	BinaryRollback bool     `json:"binary_rollback"`
}

func (m Manifest) validate(tag string) error {
	product, ok := ProductForTag(tag)
	if !ok || m.Tag != tag || m.Version != product {
		return errors.New("release manifest version does not match tag")
	}
	switch m.Migration {
	case "none", "apply", "manual":
	default:
		return errors.New("invalid release migration mode")
	}
	seen := make(map[string]bool)
	for _, version := range m.UpgradeFrom {
		if !ValidVersion(version) || Compare(version, m.Version) >= 0 || seen[version] {
			return errors.New("invalid release upgrade source")
		}
		seen[version] = true
	}
	return nil
}

func ReadManifest(path, tag string) (Manifest, error) {
	var manifest Manifest
	file, err := os.Open(path)
	if err != nil {
		return manifest, err
	}
	defer file.Close()
	if err := readStrict(file, maxManifest, &manifest); err != nil {
		return manifest, err
	}
	return manifest, manifest.validate(tag)
}

func readStrict(reader io.Reader, limit int64, value any) error {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > limit {
		return errors.New("JSON body exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("unexpected content after JSON body")
	}
	return nil
}

type ReleaseInfo struct {
	Version      string `json:"version"`
	Notes        string `json:"notes"`
	URL          string `json:"url"`
	Availability string `json:"availability"`
	Migration    bool   `json:"migration"`
}

type release struct {
	Tag        string  `json:"tag_name"`
	Notes      string  `json:"body"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Immutable  bool    `json:"immutable"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func (r release) carries(name string, maxSize int64) bool {
	for _, a := range r.Assets {
		if a.Name == name && a.Size > 0 && a.Size <= maxSize {
			return true
		}
	}
	return false
}

type candidate struct {
	info     ReleaseInfo
	manifest Manifest
	release  release
}

type releaseSource struct {
	request func(context.Context, string) (*http.Response, error)
	verify  func(context.Context, string, string) error
}

func (s releaseSource) releases(ctx context.Context) ([]release, error) {
	var releases []release
	for page := 1; page <= 10; page++ {
		response, err := s.request(ctx, fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100&page=%d", Repository, page))
		if err != nil {
			return nil, err
		}
		var batch []release
		err = decodeResponse(response, 12<<20, &batch)
		if err != nil {
			return nil, err
		}
		releases = append(releases, batch...)
		if len(batch) < 100 {
			return releases, nil
		}
	}
	return nil, errors.New("release listing exceeds pagination limit")
}

func decodeResponse(response *http.Response, limit int64, out any) error {
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > limit {
		return errors.New("GitHub response exceeds size limit")
	}
	return json.Unmarshal(body, out)
}

func (s releaseSource) find(ctx context.Context, dir, current, channel, target string) (*candidate, error) {
	ctx, finish := platform.StartFlow(ctx, "查询面板发布", "channel", channel)
	defer finish()
	releases, err := s.releases(ctx)
	if err != nil {
		return nil, err
	}
	var candidates []candidate
	for _, item := range releases {
		product, valid := ProductForTag(item.Tag)
		if !valid || item.Draft || item.Prerelease != rcPattern.MatchString(item.Tag) || (channel == "stable" && item.Prerelease) {
			continue
		}
		if Compare(product, current) <= 0 || (target != "" && target != product) {
			continue
		}
		candidates = append(candidates, candidate{info: ReleaseInfo{Version: product, Notes: limitNotes(item.Notes), URL: releasePage + item.Tag}, release: item})
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	slices.SortFunc(candidates, func(a, b candidate) int { return Compare(b.info.Version, a.info.Version) })
	for index := range candidates {
		candidate := &candidates[index]
		if err := s.inspect(ctx, dir, current, candidate); err != nil {
			return nil, err
		}
		if candidate.info.Availability == "ready" {
			return candidate, nil
		}
	}
	return &candidates[0], nil
}

func (s releaseSource) inspect(ctx context.Context, dir, current string, result *candidate) error {
	archive := archiveName(result.info.Version)
	if !result.release.Immutable || !result.release.carries(manifestName, maxManifest) ||
		!result.release.carries(checksumsName, maxManifest) || !result.release.carries(archive, maxArchive) {
		result.info.Availability = "unverified"
		return nil
	}
	work, err := os.MkdirTemp(dir, "release-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	manifestPath, err := s.verifiedAsset(ctx, result.release.Tag, manifestName, work, maxManifest)
	if err != nil {
		return err
	}
	result.manifest, err = ReadManifest(manifestPath, result.release.Tag)
	if err != nil {
		return err
	}
	result.info.Migration = result.manifest.Migration == "apply"
	switch {
	case !slices.Contains(result.manifest.UpgradeFrom, current):
		result.info.Availability = "unsupported_source"
	case result.manifest.Migration == "manual":
		result.info.Availability = "manual"
	default:
		result.info.Availability = "ready"
	}
	return nil
}

func limitNotes(notes string) string {
	const maxRunes = 8000
	runes := []rune(strings.TrimSpace(notes))
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return string(runes)
}

func archiveName(product string) string {
	return fmt.Sprintf("sub2api-panel_%s_linux_%s.tar.gz", product, runtime.GOARCH)
}

func (s releaseSource) verifiedAsset(ctx context.Context, tag, name, dir string, limit int64) (string, error) {
	ctx, finish := platform.StartFlow(ctx, "校验面板发布产物", "tag", tag, "asset", name)
	defer finish()
	result := filepath.Join(dir, name)
	response, err := s.request(ctx, "https://github.com/"+Repository+"/releases/download/"+tag+"/"+name)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub asset returned HTTP %d", response.StatusCode)
	}
	out, err := os.OpenFile(result, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	size, copyErr := io.Copy(out, io.LimitReader(response.Body, limit+1))
	if err := errors.Join(copyErr, out.Close()); err != nil {
		return "", err
	}
	if size == 0 || size > limit {
		return "", errors.New("invalid GitHub asset size")
	}
	if err := s.verify(ctx, tag, result); err != nil {
		return "", err
	}
	return result, nil
}

func githubSource() releaseSource {
	client := &http.Client{
		Timeout: 2 * time.Minute,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			host := r.URL.Host
			if len(via) >= 5 || r.URL.Scheme != "https" || r.URL.User != nil ||
				(host != "github.com" && host != "release-assets.githubusercontent.com" && host != "objects.githubusercontent.com") {
				return errors.New("invalid GitHub download redirect")
			}
			return nil
		},
	}
	return releaseSource{
		request: func(ctx context.Context, url string) (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("User-Agent", "Sub2API-Panel-Updater")
			if req.URL.Host == "api.github.com" {
				req.Header.Set("Accept", "application/vnd.github+json")
			}
			res, err := client.Do(req)
			if err == nil {
				platform.UpdateLog(ctx, "GitHub 面板发布请求", "status", res.StatusCode, "github_request_id", res.Header.Get("X-GitHub-Request-Id"))
			}
			return res, err
		},
		verify: func(ctx context.Context, tag, path string) error {
			return runCommand(ctx, "/usr/bin/gh", "release", "verify-asset", tag, path, "--repo", Repository)
		},
	}
}

func ValidateRelease(ctx context.Context, tag, manifestPath, approvedTarget string) error {
	manifest, err := ReadManifest(manifestPath, tag)
	if err != nil {
		return err
	}
	releases, err := githubSource().releases(ctx)
	if err != nil {
		return err
	}
	return validateAgainstPublished(releases, manifest, approvedTarget)
}

func validateAgainstPublished(releases []release, manifest Manifest, approvedTarget string) error {
	previous := ""
	for _, published := range releases {
		if published.Draft {
			continue
		}
		if published.Tag == manifest.Tag {
			return errors.New("Git 标签已发布")
		}
		if published.Prerelease {
			continue
		}
		product, ok := ProductForTag(published.Tag)
		if !ok {
			return fmt.Errorf("发现不符合版本规则的正式标签 %q", published.Tag)
		}
		if Compare(product, previous) > 0 {
			previous = product
		}
	}
	if rcPattern.MatchString(manifest.Tag) {
		baseline := previous
		if baseline == "" {
			baseline = FirstProduction
		}
		segments := strings.SplitN(baseline, ".", 3)
		if !strings.HasPrefix(manifest.Version, segments[0]+"."+segments[1]+".") && approvedTarget != manifest.Version {
			return errors.New("变更 RC 标签的前两段需要主人明确指定目标版本")
		}
		return nil
	}
	return ValidateProductionRelease(previous, manifest.Version, manifest.Tag, approvedTarget)
}
