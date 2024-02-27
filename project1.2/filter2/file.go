package filter2

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

var (
	//go:embed rules
	crs  embed.FS
	root fs.FS
)

func init() {
	// 从 embed.FS 中提取名为 "rules" 的子目录
	rules, _ := fs.Sub(crs, "rules")

	// 初始化一个 rulesFS，用于处理文件系统操作
	root = &rulesFS{
		fs: rules,
		filesMapping: map[string]string{
			"@recommended-conf":    "coraza.conf-recommended.conf",
			"@demo-conf":           "coraza-demo.conf",
			"@crs-setup-demo-conf": "crs-setup.conf.example",
			"@ftw-conf":            "ftw-config.conf",
			"@crs-setup-conf":      "crs-setup.conf.example",
		},
		dirsMapping: map[string]string{
			"@owasp_crs": "crs",
		},
	}
}

// rulesFS 实现了 fs.FS 接口，用于处理文件系统的操作
type rulesFS struct {
	fs           fs.FS
	filesMapping map[string]string // 文件映射关系，将虚拟路径映射到实际文件
	dirsMapping  map[string]string // 目录映射关系，将虚拟目录映射到实际目录
}

// Open 实现了 fs.FS 接口的 Open 方法
func (r rulesFS) Open(name string) (fs.File, error) {
	return r.fs.Open(r.mapPath(name))
}

// ReadDir 实现了 fs.FS 接口的 ReadDir 方法
func (r rulesFS) ReadDir(name string) ([]fs.DirEntry, error) {
	for a, dst := range r.dirsMapping {
		if a == name {
			return fs.ReadDir(r.fs, dst)
		}

		prefix := a + "/"
		if strings.HasPrefix(name, prefix) {
			return fs.ReadDir(r.fs, fmt.Sprintf("%s/%s", dst, name[len(prefix):]))
		}
	}
	return fs.ReadDir(r.fs, name)
}

// ReadFile 实现了 fs.FS 接口的 ReadFile 方法
func (r rulesFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(r.fs, r.mapPath(name))
}

// mapPath 用于根据映射关系将虚拟路径映射为实际路径
func (r rulesFS) mapPath(p string) string {
	if strings.IndexByte(p, '/') != -1 {
		// 如果不在根目录，进行目录映射
		for a, dst := range r.dirsMapping {
			prefix := a + "/"
			if strings.HasPrefix(p, prefix) {
				return fmt.Sprintf("%s/%s", dst, p[len(prefix):])
			}
		}
	}

	// 文件映射
	for a, dst := range r.filesMapping {
		if a == p {
			return dst
		}
	}

	return p
}
