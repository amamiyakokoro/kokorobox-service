//go:build !windows

package processrouter

func startNativeProcess(string, string) (nativeProcess, error) {
	return nil, ErrUnsupported
}

func hardenProcessRouterPaths(string, string) error {
	return nil
}
