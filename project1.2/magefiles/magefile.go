//go:build mage

package main

import (
	"errors"
	"fmt"
	"github.com/magefile/mage/sh"
	"github.com/tetratelabs/wabin/binary"
	"github.com/tetratelabs/wabin/wasm"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var minGoVersion = "1.20"
var minTinygoVersion = "0.30"

var errCommitFormatting = errors.New("files not formatted, please commit formatting changes")

func init() {
	for _, check := range []struct {
		lang       string
		minVersion string
	}{
		{"tinygo", minTinygoVersion},
		{"go", minGoVersion},
	} {
		if err := checkVersion(check.lang, check.minVersion); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// checkVersion checks the minimum version of the specified language is supported.
// Note: While it is likely, there are no guarantees that a newer version of the language will work
func checkVersion(lang string, minVersion string) error {
	var compare []string

	switch lang {
	case "go":
		// Version can/cannot include patch version e.g.
		// - go version go1.19 darwin/arm64
		// - go version go1.19.2 darwin/amd64
		goVersionRegex := regexp.MustCompile("go([0-9]+).([0-9]+).?([0-9]+)?")
		v, err := sh.Output("go", "version")
		if err != nil {
			return fmt.Errorf("unexpected go error: %v", err)
		}
		compare = goVersionRegex.FindStringSubmatch(v)
		if len(compare) != 4 {
			return fmt.Errorf("unexpected go semver: %q", v)
		}
	case "tinygo":
		tinygoVersionRegex := regexp.MustCompile("tinygo version ([0-9]+).([0-9]+).?([0-9]+)?")
		v, err := sh.Output("tinygo", "version")
		if err != nil {
			return fmt.Errorf("unexpected tinygo error: %v", err)
		}
		// Assume a dev build is valid.
		if strings.Contains(v, "-dev") {
			return nil
		}
		compare = tinygoVersionRegex.FindStringSubmatch(v)
		if len(compare) != 4 {
			return fmt.Errorf("unexpected tinygo semver: %q", v)
		}
	default:
		return fmt.Errorf("unexpected language: %s", lang)
	}

	compare = compare[1:]
	if compare[2] == "" {
		compare[2] = "0"
	}

	base := strings.SplitN(minVersion, ".", 3)
	if len(base) == 2 {
		base = append(base, "0")
	}
	for i := 0; i < 3; i++ {
		baseN, _ := strconv.Atoi(base[i])
		compareN, _ := strconv.Atoi(compare[i])
		if baseN > compareN {
			return fmt.Errorf("unexpected %s version, minimum want %q, have %q", lang, minVersion, strings.Join(compare, "."))
		}
	}
	return nil
}

// Build builds the Coraza wasm plugin.
func Build() error {
	if err := os.MkdirAll("build", 0755); err != nil {
		return err
	}
	//custommalloc: 使用自定义的内存分配器，可能是对标准 Go 内存分配器的替代。
	//nottinygc_envoy: 与 Envoy 集成的 TinyGo 不使用 TinyGo 的垃圾回收器（TinyGC）。
	//no_fs_access: 禁用对文件系统的访问。
	//memoize_builders: 使用 memoization 来优化规则构建。
	//coraza.rule.multiphase_evaluation: 启用 Coraza 框架的多阶段规则评估功能。
	//timing 和 proxywasm_timing: 启用与执行时间相关的功能，可能用于性能分析。
	//memstats: 启用内存统计功能，用于收集有关内存使用的信息。

	buildTags := []string{
		"custommalloc",     // https://github.com/wasilibs/nottinygc#usage
		"nottinygc_envoy",  // https://github.com/wasilibs/nottinygc#using-with-envoy
		"no_fs_access",     // https://github.com/corazawaf/coraza#build-tags
		"memoize_builders", // https://github.com/corazawaf/coraza#build-tags
	}
	// By default multiphase evaluation is enabled
	if os.Getenv("MULTIPHASE_EVAL") != "false" {
		buildTags = append(buildTags, "coraza.rule.multiphase_evaluation")
	}
	if os.Getenv("TIMING") == "true" {
		buildTags = append(buildTags, "timing", "proxywasm_timing")
	}
	if os.Getenv("MEMSTATS") == "true" {
		buildTags = append(buildTags, "memstats")
	}

	buildTagArg := fmt.Sprintf("-tags='%s'", strings.Join(buildTags, " "))

	// ~100MB initial heap
	initialPages := 2100
	if ipEnv := os.Getenv("INITIAL_PAGES"); ipEnv != "" {
		if ip, err := strconv.Atoi(ipEnv); err != nil {
			return err
		} else {
			initialPages = ip
		}
	}

	if err := sh.RunV("tinygo", "build", "-gc=custom", "-opt=2", "-o", filepath.Join("build", "mainraw.wasm"), "-scheduler=none", "-target=wasi", buildTagArg); err != nil {
		return err
	}

	return patchWasm(filepath.Join("build", "mainraw.wasm"), filepath.Join("build", "main.wasm"), initialPages)
}

func patchWasm(inPath, outPath string, initialPages int) error {
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	mod, err := binary.DecodeModule(raw, wasm.CoreFeaturesV2)
	if err != nil {
		return err
	}

	mod.MemorySection.Min = uint32(initialPages)

	for _, imp := range mod.ImportSection {
		switch {
		case imp.Name == "fd_filestat_get":
			imp.Name = "fd_fdstat_get"
		case imp.Name == "path_filestat_get":
			imp.Module = "env"
			imp.Name = "proxy_get_header_map_value"
		}
	}

	out := binary.EncodeModule(mod)
	if err = os.WriteFile(outPath, out, 0644); err != nil {
		return err
	}

	return nil
}
