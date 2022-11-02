package main

type ModLoader interface {
	GetDownloads(installPath string) []Download
	Install(installPath string, java JavaProvider) bool

	// GetLaunchJar
	// First return parameter describes the 'Main Jar'.
	// Second return parameter describes a list of JVM arguments.
	// If 'Main Jar' is empty, it is expected that the JVM arg list
	// contains classpath/main class entries (Modular Forge 1.17+)
	GetLaunchJar(installPath string) (string, []string)
}
