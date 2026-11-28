package cli

import (
	"context"
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
	"time"

	"sub2api-panel/internal/panelupdate"
)

type UpdateManifest struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	SHA256  string `json:"sha256"`
}

func UpdateCLI(args []string, version string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print("update check --manifest 清单.json --file 候选二进制\nupdate install --manifest 清单.json --file 候选二进制 --target 已安装的二进制\nupdate rollback --target 已安装的二进制\nupdate executor\nupdate product-version <tag>\nupdate validate-release <tag> <manifest>\n")
		return nil
	}
	switch args[0] {
	case "executor":
		if len(args) != 1 {
			return fmt.Errorf("更新执行器不接受额外参数")
		}
		return panelupdate.RunExecutor()
	case "product-version":
		if len(args) == 2 {
			if product, ok := panelupdate.ProductForTag(args[1]); ok {
				fmt.Println(product)
				return nil
			}
		}
		return fmt.Errorf("无效的发布 Git 标签")
	case "validate-release":
		if len(args) != 3 {
			return fmt.Errorf("请指定 Git 标签和发布清单")
		}
		return validateRelease(args[1], args[2])
	case "validate-build":
		if len(args) != 3 || (args[2] != "true" && args[2] != "false") {
			return fmt.Errorf("请指定 Git 标签和快照状态")
		}
		if args[2] == "true" {
			return nil
		}
		product, ok := panelupdate.ProductForTag(args[1])
		if !ok || product != os.Getenv("PANEL_PRODUCT_VERSION") {
			return fmt.Errorf("PANEL_PRODUCT_VERSION 必须与 Git 标签对应")
		}
		return validateRelease(args[1], "release-input/update-manifest.json")
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
			fmt.Printf("当前版本 %s，候选版本 %s，平台与 SHA-256 校验通过，存在新版本 %t\n", version, candidate.Version, panelupdate.Compare(candidate.Version, version) > 0)
			return nil
		}
		return InstallUpdate(*file, *target, candidate)
	case "rollback":
		return RollbackUpdate(*target)
	default:
		return fmt.Errorf("未知更新命令 %q", args[0])
	}
}

func validateRelease(tag, manifest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return panelupdate.ValidateRelease(ctx, tag, manifest, os.Getenv("PANEL_APPROVED_VERSION_TARGET"))
}

func readUpdateManifest(path string) (UpdateManifest, error) {
	var result UpdateManifest
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
	if err != nil || len(checksum) != sha256.Size || !panelupdate.ValidVersion(result.Version) || result.OS != runtime.GOOS || result.Arch != runtime.GOARCH {
		return result, fmt.Errorf("发布清单与当前平台不匹配")
	}
	return result, nil
}

func copyCheckedBinary(source string, dest io.Writer, manifest UpdateManifest) (int64, error) {
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

func InstallUpdate(source, target string, manifest UpdateManifest) error {
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

func RollbackUpdate(target string) error {
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
