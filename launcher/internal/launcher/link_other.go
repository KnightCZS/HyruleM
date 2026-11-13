//go:build !windows

package launcher

import "os"

func directoryLink(link, target string) error { return os.Symlink(target, link) }
