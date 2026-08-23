package obs

import (
	"runtime"
	"strconv"
	"strings"
)

// selfPath is this file, as the compiler recorded it. Used to work out how much of every
// caller path is machine-specific noise.
const selfPath = "web/obs/caller.go"

// callerPrefix is the absolute path of the repository root the binary was built from, or ""
// when it cannot be determined — in which case caller paths are left exactly as they are
// rather than mangled by guesswork.
var callerPrefix = func() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok || !strings.HasSuffix(file, selfPath) {
		return ""
	}
	return strings.TrimSuffix(file, selfPath)
}()

// RepoRelativeCaller renders the caller field as a path inside the repository:
// gitlab/archive.go:47, not /home/whoever/src/glabs/gitlab/... . Assign it to
// zerolog.CallerMarshalFunc wherever the logger is configured.
//
// zerolog's default is the compiler's absolute path, which is the BUILD machine's, so the
// same line reads differently in the container than it does here. That matters more than
// tidiness: scrub.go fingerprints log events on this field, so with the default a bug
// reproduced locally lands in a different issue than the production one it was meant to
// explain — silently, because both look perfectly fine on their own.
//
// It lives HERE rather than in bootstrap because the package that depends on the format
// should own it: anything that reports — including this package's own tests — then gets the
// right paths without having to remember a second call.
func RepoRelativeCaller(_ uintptr, file string, line int) string {
	if callerPrefix != "" {
		file = strings.TrimPrefix(file, callerPrefix)
	}
	return file + ":" + strconv.Itoa(line)
}
