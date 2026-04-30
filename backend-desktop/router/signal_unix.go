//go:build darwin || linux

package router

import "syscall"

// zeroSignal 在 unix 上是 syscall.Signal(0)，用于无副作用地探测 PID 是否仍然存活。
var zeroSignal = syscall.Signal(0)
