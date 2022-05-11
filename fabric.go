package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
)

const FABRIC_META_API = "https://meta.fabricmc.net/v2/versions/loader/%s/%s/server/json"

type FabricVersion struct {
	RawVersion string
	Minecraft  Minecraft
}

type Fabric struct {
	FabricVersion
	metaCache FabricMeta
}

type FabricMeta struct {
	Libraries []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"libraries"`
	MainClass string `json:"mainClass"`
}

func (f *Fabric) getMeta() FabricMeta {
	if len(f.metaCache.Libraries) == 0 {
		var meta = FabricMeta{}
		var url = fmt.Sprintf(FABRIC_META_API, f.Minecraft.RawVersion, f.RawVersion)
		resp, err := http.Get(url)
		if err != nil {
			log.Fatalf("Error getting fabric meta for Minecraft %s Fabric %s: %v", f.Minecraft.RawVersion, f.RawVersion, err)
		}

		defer resp.Body.Close()
		rawFabricBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error getting fabric meta for Minecraft %s Fabric %s: %v", f.Minecraft.RawVersion, f.RawVersion, err)
		}

		err = json.Unmarshal(rawFabricBytes, &meta)
		if err != nil {
			log.Fatalf("Error parsing fabric meta for Minecraft %s Fabric %s: %v", f.Minecraft.RawVersion, f.RawVersion, err)
		}

		f.metaCache = meta
	}

	return f.metaCache
}

func (f Fabric) GetDownloads(installPath string) []Download {
	log.Println("Getting downloads for Fabric")
	vanillaVer, err := f.FabricVersion.Minecraft.GetVanillaVersion()
	if err != nil {
		// handleerr
	}

	serverDownload, err := vanillaVer.GetServerDownload()
	if err != nil {
		// handleerr
	}

	downloads := []Download{serverDownload}

	meta := f.getMeta()
	homeDir := getFabricHomeDir()
	for _, library := range meta.Libraries {
		mavenURL, filename := getMavenUrl(library.Name)
		mavenURL = GetMirrorFor(mavenURL, library.URL)
		sha1URL := mavenURL + ".sha1"
		sha1 := getOrBlank(sha1URL)

		parse, err := url.Parse(mavenURL)
		if err != nil {
			//shrug
			continue
		}

		downloads = append(downloads, Download{path.Join(homeDir, ".cache"), *parse, filename, "sha1", sha1, path.Join(homeDir, ".cache", filename)})
	}
	return downloads
}

func (f Fabric) Install(installPath string) bool {
	log.Println("Installing Fabric")
	serverName := fmt.Sprintf("fabric-%s-%s-server-launch.jar", f.Minecraft.RawVersion, f.FabricVersion.RawVersion)
	meta := f.getMeta()

	downloads := f.GetDownloads(installPath)

	var jars []string

	for _, download := range downloads {
		if strings.Contains(download.FullPath, ".cache") {
			jars = append(jars, download.FullPath)
		}
	}

	vanillaVer, err := f.FabricVersion.Minecraft.GetVanillaVersion()
	if err != nil {
		// handleerr
	}

	serverDownload, err := vanillaVer.GetServerDownload()
	if err != nil {
		// handleerr
	}

	ioutil.WriteFile(path.Join(installPath, "fabric-server-launcher.properties"), []byte("serverJar="+serverDownload.Name+"\n"), 0644)

	mergeZips(jars, path.Join(installPath, serverName), false, meta.MainClass)

	return true
}

func (f Fabric) GetLaunchJar(installPath string) string {
	return fmt.Sprintf("fabric-%s-%s-server-launch.jar", f.Minecraft.RawVersion, f.FabricVersion.RawVersion)
}

func GetFabric(modloader Target, mc Minecraft) (error, ModLoader) {
	fab := Fabric{}
	fab.FabricVersion.RawVersion = modloader.Version
	fab.FabricVersion.Minecraft = mc
	return nil, fab
}

func getMavenUrl(artifact string) (string, string) {
	split := strings.Split(artifact, ":")
	packge := split[0]
	thing := split[1] // I can't remember the name, ok
	thingEscaped := url.QueryEscape(thing)
	version := split[2]
	versionEscaped := url.QueryEscape(version)

	packgeURL := strings.Replace(packge, ".", "/", -1)

	jarURL := fmt.Sprintf("%s/%s/%s/%s-%s.jar", packgeURL, thingEscaped, versionEscaped, thing, version)

	filename := fmt.Sprintf("%s-%s.jar", thing, version)

	return jarURL, filename
}

func getFabricHomeDir() string { // try to mimic fabric installer cache behaviour
	var home string
	var err error
	switch runtime.GOOS {
	case "windows", "darwin", "ios":
		home, err = os.UserConfigDir()
	default:
		home, err = os.UserHomeDir()
	}
	if err != nil {
		return ""
	} else {
		return home
	}
}
