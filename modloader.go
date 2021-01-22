package main

type modLoader interface {
	getDownloads(installPath string) []Download
	install(installPath string) bool
	getLaunchJar(installPath string) string
}
