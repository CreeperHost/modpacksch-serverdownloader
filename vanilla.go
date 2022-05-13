package main

import (
	"fmt"
	"log"
)

func (m Minecraft) GetDownloads(installPath string) []Download {
	log.Println("Getting downloads for Vanilla")
	vanillaVer, err := m.GetVanillaVersion()
	if err != nil {
		// handleerr
	}

	serverDownload, err := vanillaVer.GetServerDownload()
	if err != nil {
		// handleerr
	}

	return []Download{serverDownload}
}

func (m Minecraft) Install(installPath string) bool {
	return true
}

func (m Minecraft) GetLaunchJar(installPath string) (string, []string) {
	return fmt.Sprintf("minecraft_server.%s.jar", m.RawVersion), nil
}
