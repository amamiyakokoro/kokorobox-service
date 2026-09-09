//go:build !windows && !linux

package processrouter

func startNativeProcess(string, string) (nativeProcess, error) {
	return nil, ErrUnsupported
}

func hardenProcessRouterPaths(string, string) error {
	return nil
}
