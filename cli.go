package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

var version = "dev"
var commit = "unknown"

const cliHelp = `Sub2API 管理面板

用法：sub2api-panel <命令>

  serve             启动面板
  issuer            启动发码进程
  migrate           执行 PostgreSQL 迁移
  --version         显示版本和构建信息
  update check      核对下载后的候选二进制
  update install    安装已经校验的候选二进制
  update rollback   恢复上次安装前的二进制

使用 update <命令> --help 查看参数。
`

func runCLI(args []string) (handled bool, code int) {
	if len(args) == 1 {
		switch args[0] {
		case "serve", "issuer", "migrate":
			return false, 0
		case "--version", "version":
			fmt.Printf("sub2api-panel %s (%s, %s/%s)\n", version, commit, runtime.GOOS, runtime.GOARCH)
			return true, 0
		case "--help", "-h", "help":
			fmt.Print(cliHelp)
			return true, 0
		}
	}
	if len(args) > 0 && args[0] == "update" {
		if err := updateCLI(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return true, 1
		}
		return true, 0
	}
	fmt.Fprint(os.Stderr, cliHelp)
	return true, 2
}

type updateManifest struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	SHA256  string `json:"sha256"`
}

func updateCLI(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print("update check --manifest 清单.json --file 候选二进制\nupdate install --manifest 清单.json --file 候选二进制 --target 已安装的二进制\nupdate rollback --target 已安装的二进制\n")
		return nil
	}
	fs := flag.NewFlagSet("update "+args[0], flag.ContinueOnError)
	manifest := fs.String("manifest", "", "发布清单")
	file := fs.String("file", "", "候选二进制")
	target := fs.String("target", "", "已安装的二进制")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("更新命令有多余参数")
	}
	switch args[0] {
	case "check", "install":
		candidate, err := readUpdateManifest(*manifest)
		if err != nil {
			return err
		}
		if args[0] == "check" {
			if _, err := copyCheckedBinary(*file, io.Discard, candidate); err != nil {
				return err
			}
			fmt.Printf("当前版本 %s，候选版本 %s，平台与 SHA-256 校验通过，存在新版本 %t\n", version, candidate.Version, semver.Compare(asSemver(candidate.Version), asSemver(version)) > 0)
			return nil
		}
		return installUpdate(*file, *target, candidate)
	case "rollback":
		return rollbackUpdate(*target)
	default:
		return fmt.Errorf("未知更新命令 %q", args[0])
	}
}

func asSemver(value string) string {
	if !strings.HasPrefix(value, "v") {
		return "v" + value
	}
	return value
}

func readUpdateManifest(path string) (updateManifest, error) {
	var result updateManifest
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF {
		return result, fmt.Errorf("发布清单格式无效")
	}
	checksum, err := hex.DecodeString(result.SHA256)
	if err != nil || len(checksum) != sha256.Size || !semver.IsValid(asSemver(result.Version)) || result.OS != runtime.GOOS || result.Arch != runtime.GOARCH {
		return result, fmt.Errorf("发布清单与当前平台不匹配")
	}
	return result, nil
}

func copyCheckedBinary(source string, dest io.Writer, manifest updateManifest) (int64, error) {
	binary, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer binary.Close()
	info, err := binary.Stat()
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 512<<20 {
		return 0, fmt.Errorf("候选二进制格式或大小无效")
	}
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(dest, hash), io.LimitReader(binary, info.Size()+1))
	if err != nil {
		return 0, err
	}
	if size != info.Size() || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), manifest.SHA256) {
		return 0, fmt.Errorf("候选二进制 SHA-256 校验失败")
	}
	return size, nil
}

func installUpdate(source, target string, manifest updateManifest) error {
	return withUpdateLock(target, func(path string, info os.FileInfo) error {
		candidate, err := os.Open(source)
		if err != nil {
			return err
		}
		sourceInfo, err := candidate.Stat()
		candidate.Close()
		if err != nil {
			return err
		}
		if os.SameFile(info, sourceInfo) {
			return fmt.Errorf("候选二进制与安装目标相同")
		}
		tmp, err := os.CreateTemp(filepath.Dir(path), ".panel-update-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		_, copyErr := copyCheckedBinary(source, tmp, manifest)
		configureErr := errors.Join(tmp.Chmod(info.Mode().Perm()), tmp.Sync(), tmp.Close())
		if err := errors.Join(copyErr, configureErr); err != nil {
			return err
		}
		previous := path + ".previous"
		if err := copyUpdateBackup(path, previous, info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Rename(tmp.Name(), path); err != nil {
			return err
		}
		fmt.Println("已安装 " + manifest.Version + "，可由服务管理器启动新版本")
		return nil
	})
}

func rollbackUpdate(target string) error {
	return withUpdateLock(target, func(path string, _ os.FileInfo) error {
		previous := path + ".previous"
		info, err := os.Lstat(previous)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("备份二进制无效")
		}
		if err := os.Rename(previous, path); err != nil {
			return err
		}
		fmt.Println("已恢复上次安装前的二进制")
		return nil
	})
}

func copyUpdateBackup(path, previous string, mode os.FileMode) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".panel-previous-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, copyErr := io.Copy(tmp, src)
	if err := errors.Join(copyErr, tmp.Chmod(mode), tmp.Sync(), tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), previous)
}

func withUpdateLock(target string, run func(string, os.FileInfo) error) error {
	if !filepath.IsAbs(target) {
		return fmt.Errorf("请用 --target 指定安装二进制的绝对路径")
	}
	target = filepath.Clean(target)
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("安装目标必须是普通文件")
	}
	lock, err := os.OpenFile(target+".update-lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	lock.Close()
	defer os.Remove(lock.Name())
	return run(target, info)
}
