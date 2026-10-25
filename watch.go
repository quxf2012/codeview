package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	watchInterval  = time.Second
	watchHeartbeat = 20 * time.Second
	maxWatchPaths  = 300
)

// handleWatch 用 Server-Sent Events 推送文件变化。
//
// 浏览器把当前文件、当前目录和左侧树里展开的目录作为 ?p= 传进来，
// 服务端每秒检查一次（文件看 mtime+大小；目录看列表里各项的名字、大小和修改时间），
// 有变化就推送 {"path":..., "state":"changed"|"deleted"}。
// 用轮询而不是 inotify：没有额外依赖，网络文件系统、虚拟机共享目录上 inotify 也不可靠，
// 只盯着正在看的这几个路径，开销可以忽略。
func (s *server) handleWatch(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	paths := r.URL.Query()["p"]
	if len(paths) > maxWatchPaths {
		paths = paths[:maxWatchPaths]
	}

	// 签名：文件取大小+修改时间；目录取列表（含子项大小和修改时间）的哈希，
	// 这样目录页里的大小/时间也能跟着文件内容的修改刷新。不存在时为空串。
	stat := func(p string) string {
		rel, abs, err := s.resolve(p)
		if err != nil {
			return ""
		}
		info, err := os.Stat(abs)
		if err != nil {
			return ""
		}
		if !info.IsDir() {
			return fmt.Sprintf("f%d:%d", info.Size(), info.ModTime().UnixNano())
		}
		entries, err := s.listDir(rel, abs)
		if err != nil {
			return ""
		}
		h := fnv.New64a()
		for _, e := range entries {
			fmt.Fprintf(h, "%s\x00%s\x00%d\x00%d\n", e.Name, e.Type, e.Size, e.ModTime.UnixNano())
		}
		return "d" + strconv.FormatUint(h.Sum64(), 16)
	}

	last := make(map[string]string, len(paths))
	for _, p := range paths {
		last[p] = stat(p)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, "retry: 2000\n: ok\n\n")
	flusher.Flush()

	tick := time.NewTicker(watchInterval)
	defer tick.Stop()
	beat := time.NewTicker(watchHeartbeat)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-beat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-tick.C:
			sent := false
			for _, p := range paths {
				cur := stat(p)
				if cur == last[p] {
					continue
				}
				last[p] = cur
				state := "changed"
				if cur == "" {
					state = "deleted"
				}
				b, _ := json.Marshal(map[string]string{"path": p, "state": state})
				fmt.Fprintf(w, "data: %s\n\n", b)
				sent = true
			}
			if sent {
				flusher.Flush()
			}
		}
	}
}
