package gitlab

import "os"

var exitFunc = os.Exit

var panicFunc = func(v interface{}) {
	panic(v)
}
