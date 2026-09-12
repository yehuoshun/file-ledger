// openBrowser 跨平台打开默认浏览器。
// Windows 用 rundll32（cmd /c start 也行，rundll32 更稳）；
// Linux/macOS 用 xdg-open/open 兜底（开发自测用）。
// 注意：浏览器进程会被启动器回收，属正常现象，服务进程不受影响。
package main

import (
	"log"
	"os/exec"
	"runtime"
)

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("打开浏览器失败（不影响服务）: %v", err)
	}
}
