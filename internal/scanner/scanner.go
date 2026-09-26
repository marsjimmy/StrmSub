// Package scanner 在媒体目录中找出 .strm 文件。
package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// FindStrm 递归扫描 dirs，返回所有 .strm 文件路径（大小写不敏感）。
func FindStrm(dirs []string) ([]string, error) {
	var out []string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // 跳过不可读目录，不中断整轮扫描
			}
			if d.IsDir() {
				// 跳过各家 NAS 的缩略图/缓存目录
				name := d.Name()
				if strings.HasPrefix(name, "@") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "#") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".strm") {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
