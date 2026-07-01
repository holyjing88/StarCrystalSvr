// Package logger 提供进程内唯一日志入口；业务与中间件均通过本包 API 输出，具体输出目标由 Init 配置。
package logger

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// 统一主题名，便于检索与约定格式 [topic]。
const (
	TopicAPI  = "api"
	TopicAuth = "auth"
	TopicMain = "main"
)

// DebugEnabled 为 true 时表示当前日志级别会输出 Debug（可避免热路径上无意义的序列化）。
func DebugEnabled() bool {
	return currentLevel() <= LevelDebug
}

var (
	mu      sync.Mutex
	writeMu sync.Mutex
	out     *log.Logger
	level   Level = LevelDebug
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelError
	LevelFatal
)

func ensure() *log.Logger {
	mu.Lock()
	defer mu.Unlock()
	if out == nil {
		out = log.New(os.Stdout, "", log.LstdFlags)
	}
	return out
}

// Init 将日志输出定向到指定 Writer（例如 stdout+日文件 MultiWriter）。须在进程早期调用一次。
func Init(w io.Writer, lv string) {
	mu.Lock()
	defer mu.Unlock()
	out = log.New(w, "", log.LstdFlags)
	level = ParseLevel(lv)
}

// SvrStartBanner 写入一条启动分隔行（不受日志级别门控；不含 ][warn]/error/fatal，故仅写入主日志文件与 stdout）。
// 用于在日志中区分「本次进程」的起始位置。
func SvrStartBanner() {
	printfSafe("===============svr start %s=================", time.Now().Format("2006-01-02 15:04:05"))
}

func ParseLevel(raw string) Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fatal":
		return LevelFatal
	case "error":
		return LevelError
	case "info":
		return LevelInfo
	case "debug", "":
		return LevelDebug
	default:
		return LevelDebug
	}
}

func currentLevel() Level {
	mu.Lock()
	defer mu.Unlock()
	return level
}

// Printf 与 Info 同级别门控（LevelInfo）；无 topic 前缀，仅用极早期或未引入 TopicMain 的场景。
func Printf(format string, args ...interface{}) {
	if currentLevel() > LevelInfo {
		return
	}
	printfSafe("[info] "+format, args...)
}

// Info 输出 [topic] 前缀的常规信息。
func Info(topic, format string, args ...interface{}) {
	if currentLevel() > LevelInfo {
		return
	}
	printfSafe("["+topic+"][info] "+format, args...)
}

// Debug 输出 [topic][debug] 调试信息。
func Debug(topic, format string, args ...interface{}) {
	if currentLevel() > LevelDebug {
		return
	}
	printfSafe("["+topic+"][debug] "+format, args...)
}

// Warn 输出 [topic][warn]。
func Warn(topic, format string, args ...interface{}) {
	if currentLevel() > LevelInfo {
		return
	}
	printfSafe("["+topic+"][warn] "+format, args...)
}

// Error 输出 [topic][error]。
func Error(topic, format string, args ...interface{}) {
	if currentLevel() > LevelError {
		return
	}
	printfSafe("["+topic+"][error] "+format, args...)
}

// InfoTrace 输出 [topic] [trace=id] ...；trace 为空时退化为 Info。
func InfoTrace(traceID, topic, format string, args ...interface{}) {
	if traceID == "" {
		Info(topic, format, args...)
		return
	}
	if currentLevel() > LevelInfo {
		return
	}
	printfSafe("["+topic+"][info] [trace=%s] "+format, append([]interface{}{traceID}, args...)...)
}

// DebugTrace 输出 [topic][debug] [trace=id] ...；trace 为空时退化为 Debug。
// 与 Debug 一致：仅当日志级别为 debug 时输出（避免 info 模式下仍刷屏）。
func DebugTrace(traceID, topic, format string, args ...interface{}) {
	if currentLevel() > LevelDebug {
		return
	}
	if traceID == "" {
		Debug(topic, format, args...)
		return
	}
	printfSafe("["+topic+"][debug] [trace=%s] "+format, append([]interface{}{traceID}, args...)...)
}

// FatalNotice writes [topic][fatal] ... without exiting and without level filtering
// (for startup banners that must appear even when LOG_LEVEL=fatal). Use sparingly.
func FatalNotice(topic, format string, args ...interface{}) {
	printfSafe("["+topic+"][fatal] "+format, args...)
}

// Fatal 输出后以非零退出；等同于原 log.Fatal，但带 topic 前缀。
func Fatal(topic, format string, args ...interface{}) {
	fatalfSafe("["+topic+"][fatal] "+format, args...)
}

func printfSafe(format string, args ...interface{}) {
	l := ensure()
	writeMu.Lock()
	defer writeMu.Unlock()
	l.Printf(format, args...)
}

func fatalfSafe(format string, args ...interface{}) {
	l := ensure()
	writeMu.Lock()
	defer writeMu.Unlock()
	l.Fatalf(format, args...)
}
