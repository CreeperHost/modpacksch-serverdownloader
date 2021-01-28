package main

type ModLoader interface {
	GetDownloads(installPath string) []Download
	Install(installPath string) bool
	GetLaunchJar(installPath string) string
}
