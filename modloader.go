package main

import (
	"errors"
	"fmt"
)

func GetModloader(modloader Target, mc Minecraft) (error, ModLoader) {
	if modloader.Name == "forge" {
		return GetForge(modloader, mc)
	}
	return errors.New(fmt.Sprintf("Unable to locate modloader for %s %s %s", modloader.Name, modloader.Version, mc.RawVersion)), nil
}

type ModLoader interface {
	GetDownloads(installPath string) []Download
	Install(installPath string) bool
}
