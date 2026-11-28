package panelupdate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

const FirstProduction = "0.1.0001"

var productionPattern = regexp.MustCompile(`^([0-9])\.(0|[1-9][0-9]?)\.([0-9]{4})$`)
var stableTagPattern = regexp.MustCompile(`^v([0-9])\.(0|[1-9][0-9]?)\.([1-9][0-9]{0,3})$`)
var rcPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+-rc\.[1-9][0-9]*$`)

type productionVersion struct{ major, minor, patch int }

func parseProduction(value string) (productionVersion, bool) {
	parts := productionPattern.FindStringSubmatch(value)
	if parts == nil || parts[3] == "0000" {
		return productionVersion{}, false
	}
	major, _ := strconv.Atoi(parts[1])
	minor, _ := strconv.Atoi(parts[2])
	patch, _ := strconv.Atoi(parts[3])
	return productionVersion{major, minor, patch}, true
}

func (v productionVersion) tag() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

func ProductForTag(tag string) (string, bool) {
	if parts := stableTagPattern.FindStringSubmatch(tag); parts != nil {
		patch, _ := strconv.Atoi(parts[3])
		return fmt.Sprintf("%s.%s.%04d", parts[1], parts[2], patch), true
	}
	if rcPattern.MatchString(tag) && semver.IsValid(tag) {
		return strings.TrimPrefix(tag, "v"), true
	}
	return "", false
}

func normalizeVersion(value string) string {
	if version, ok := parseProduction(value); ok {
		return version.tag()
	}
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	if semver.IsValid(value) {
		return value
	}
	return ""
}

func ValidVersion(value string) bool { return normalizeVersion(value) != "" }

func Compare(left, right string) int {
	return semver.Compare(normalizeVersion(left), normalizeVersion(right))
}

func ValidateProductionRelease(previous, product, tag, approvedTarget string) error {
	next, ok := parseProduction(product)
	if !ok || tag != next.tag() {
		return fmt.Errorf("正式版本号与 Git 标签不匹配或超出范围")
	}
	if previous == "" {
		if product != FirstProduction {
			return fmt.Errorf("首个正式版必须为 %s", FirstProduction)
		}
		return nil
	}
	last, ok := parseProduction(previous)
	if !ok || Compare(product, previous) <= 0 {
		return fmt.Errorf("正式版本必须大于上一正式版本")
	}
	if next.major != last.major || next.minor != last.minor {
		if approvedTarget != product {
			return fmt.Errorf("变更前两段需要主人明确指定目标版本")
		}
		return nil
	}
	if next.patch != last.patch+1 {
		return fmt.Errorf("常规发布只能把末段加 1")
	}
	return nil
}
