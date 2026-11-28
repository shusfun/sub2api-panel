package panelupdate

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxArchive = 128 << 20
const maxBinary = 256 << 20
const binaryName = "sub2api-panel"
const grantsName = "postgres-grants.sql"

func (s releaseSource) prepare(ctx context.Context, dir string, c *candidate) error {
	sumfile, err := s.verifiedAsset(ctx, c.release.Tag, checksumsName, dir, maxManifest)
	if err != nil {
		return err
	}
	archive, err := s.verifiedAsset(ctx, c.release.Tag, archiveName(c.info.Version), dir, maxArchive)
	if err != nil {
		return err
	}
	if err := checkArchive(archive, sumfile); err != nil {
		return err
	}
	return extractRelease(archive, dir)
}

func checkArchive(archive, sumfile string) error {
	sums, err := os.Open(sumfile)
	if err != nil {
		return err
	}
	defer sums.Close()
	var expected string
	lines := bufio.NewScanner(sums)
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != filepath.Base(archive) {
			continue
		}
		hash, err := hex.DecodeString(fields[0])
		if err != nil || len(hash) != sha256.Size || expected != "" {
			return errors.New("invalid or duplicate SHA-256 entry for release archive")
		}
		expected = strings.ToLower(fields[0])
	}
	if err := lines.Err(); err != nil {
		return err
	}
	if expected == "" {
		return errors.New("release archive SHA-256 is missing")
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("release archive SHA-256 mismatch")
	}
	return nil
}

func extractRelease(archive, dir string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	// Bound all decompressed data. The release should only contain the binary and small documents.
	limited := &io.LimitedReader{R: gz, N: maxBinary + (2 << 20)}
	tarball := tar.NewReader(limited)
	found := make(map[string]bool, 2)
	for {
		header, err := tarball.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(header.Name, "./")
		var limit int64
		var mode os.FileMode
		switch name {
		case binaryName:
			limit, mode = maxBinary, 0755
		case grantsName:
			limit, mode = 128<<10, 0644
		default:
			continue
		}
		if found[name] || header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > limit {
			return errors.New("invalid or duplicate release file")
		}
		target, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(target, tarball)
		if err := errors.Join(copyErr, target.Sync(), target.Close()); err != nil {
			return err
		}
		found[name] = true
	}
	if _, err := io.Copy(io.Discard, limited); err != nil {
		return err
	}
	if limited.N == 0 || !found[binaryName] || !found[grantsName] {
		return errors.New("release archive exceeds size limit or is missing a required file")
	}
	return nil
}
