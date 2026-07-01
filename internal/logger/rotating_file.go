package logger

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	rotatingMainLogFile = "starcrystalsvr.log"
	rotatingErrLogFile  = "starcrystalsvr_error.log"
	rotatingMaxBytes    = 10 * 1024 * 1024
)

// rotatingSink duplicates each log line to os.Stdout, rotatingMainLogFile (all lines),
// and rotatingErrLogFile (warn / error / fatal lines only, including [fatal] from FatalNotice).
// When either on-disk log file exceeds rotatingMaxBytes, both files are renamed with
// a YYYYMMDDHHmm stamp (suffix before .log) and recreated empty.
// Log directory is chosen by the caller (cmd/api uses <starcrystalsvr.exe>/log).
type rotatingSink struct {
	dir string
	mu  sync.Mutex
	out *os.File
	err *os.File
}

// NewRotatingDualFileWriter creates a writer for Init(). It opens or creates both log files under logDir.
func NewRotatingDualFileWriter(logDir string) (*rotatingSink, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	s := &rotatingSink{dir: logDir}
	if err := s.openAppendEmpty(); err != nil {
		return nil, err
	}
	return s, nil
}

func rotatedPath(dir, baseFile, stamp string) string {
	ext := filepath.Ext(baseFile)
	stem := strings.TrimSuffix(baseFile, ext)
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", stem, stamp, ext))
}

func (s *rotatingSink) mainPath() string  { return filepath.Join(s.dir, rotatingMainLogFile) }
func (s *rotatingSink) errPath() string   { return filepath.Join(s.dir, rotatingErrLogFile) }
func (s *rotatingSink) pickStamp() string {
	base := time.Now().Format("200601021504")
	for i := 0; i < 200; i++ {
		suf := base
		if i > 0 {
			suf = fmt.Sprintf("%s_%02d", base, i)
		}
		p1 := rotatedPath(s.dir, rotatingMainLogFile, suf)
		p2 := rotatedPath(s.dir, rotatingErrLogFile, suf)
		if fileNotExists(p1) && fileNotExists(p2) {
			return suf
		}
	}
	return fmt.Sprintf("%s_%d", base, time.Now().Unix())
}

func fileNotExists(p string) bool {
	_, err := os.Stat(p)
	return os.IsNotExist(err)
}

func (s *rotatingSink) openAppendEmpty() error {
	return s.openPair(os.O_CREATE | os.O_APPEND | os.O_WRONLY)
}

func (s *rotatingSink) openTruncEmpty() error {
	return s.openPair(os.O_CREATE | os.O_TRUNC | os.O_WRONLY)
}

func (s *rotatingSink) openPair(flag int) error {
	mp := s.mainPath()
	ep := s.errPath()
	mf, err := os.OpenFile(mp, flag, 0644)
	if err != nil {
		return err
	}
	ef, err := os.OpenFile(ep, flag, 0644)
	if err != nil {
		_ = mf.Close()
		return err
	}
	if s.out != nil {
		_ = s.out.Close()
	}
	if s.err != nil {
		_ = s.err.Close()
	}
	s.out = mf
	s.err = ef
	return nil
}

func (s *rotatingSink) fileSizes() (mainSize int64, errSize int64) {
	if st, e := os.Stat(s.mainPath()); e == nil {
		mainSize = st.Size()
	}
	if st, e := os.Stat(s.errPath()); e == nil {
		errSize = st.Size()
	}
	return mainSize, errSize
}

func (s *rotatingSink) maybeRotateLocked() error {
	if s.out == nil || s.err == nil {
		return s.openAppendEmpty()
	}
	mSz, eSz := s.fileSizes()
	if mSz < rotatingMaxBytes && eSz < rotatingMaxBytes {
		return nil
	}
	_ = s.out.Sync()
	_ = s.err.Sync()
	mSz, eSz = s.fileSizes()
	if mSz <= rotatingMaxBytes && eSz <= rotatingMaxBytes {
		return nil
	}

	_ = s.out.Close()
	_ = s.err.Close()
	s.out, s.err = nil, nil

	stamp := s.pickStamp()
	_ = os.Rename(s.mainPath(), rotatedPath(s.dir, rotatingMainLogFile, stamp))
	_ = os.Rename(s.errPath(), rotatedPath(s.dir, rotatingErrLogFile, stamp))

	if err := s.openTruncEmpty(); err != nil {
		return s.openAppendEmpty()
	}
	return nil
}

func isElevatedLogLine(p []byte) bool {
	return bytes.Contains(p, []byte("][warn]")) ||
		bytes.Contains(p, []byte("][error]")) ||
		bytes.Contains(p, []byte("][fatal]"))
}

// Write implements io.Writer. Safe for concurrent use from multiple log.Logger callers.
func (s *rotatingSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.maybeRotateLocked()

	_, _ = os.Stdout.Write(p)
	if s.out == nil || s.err == nil {
		return len(p), nil
	}
	n, werr := s.out.Write(p)
	if isElevatedLogLine(p) {
		_, _ = s.err.Write(p)
	}
	return n, werr
}
