package service

import "runtime"

func defaultJSWorkerSocketPathForOS() string {
	if runtime.GOOS == "windows" {
		// Windows 上默认使用本地 TCP，避免 unix socket 兼容性差异。
		return "tcp://127.0.0.1:17345"
	}
	return "/tmp/auralogic-jsworker.sock"
}
