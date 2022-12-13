package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// const FABRIC_META_API = "https://meta.fabricmc.net/v2/versions/loader/%s/%s/server/json"
const (
	FABRIC_META       = "https://meta.fabricmc.net/"
	FABRIC_SERVER_JAR = "v2/versions/loader/%s/%s/%s/server/jar"
)

type FabricVersion struct {
	RawVersion string
	Minecraft  Minecraft
}

type FabricMetaInstaller []struct {
	URL     string `json:"url"`
	Maven   string `json:"maven"`
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

type Fabric struct {
	FabricVersion
	metaCache      FabricMeta
	InstallerCache FabricMetaInstaller
}

type FabricMeta struct {
	Libraries []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"libraries"`
	MainClass string `json:"mainClass"`
}

//func (f *Fabric) getMeta() FabricMeta {
//	if len(f.metaCache.Libraries) == 0 {
//		var meta = FabricMeta{}
//		var url = fmt.Sprintf(FABRIC_META_API, f.Minecraft.RawVersion, f.RawVersion)
//		resp, err := http.Get(url)
//		if err != nil {
//			log.Fatalf("Error getting fabric meta for Minecraft %s Fabric %s: %v", f.Minecraft.RawVersion, f.RawVersion, err)
//		}
//
//		defer resp.Body.Close()
//		rawFabricBytes, err := io.ReadAll(resp.Body)
//		if err != nil {
//			log.Fatalf("Error getting fabric meta for Minecraft %s Fabric %s: %v", f.Minecraft.RawVersion, f.RawVersion, err)
//		}
//
//		err = json.Unmarshal(rawFabricBytes, &meta)
//		if err != nil {
//			log.Fatalf("Error parsing fabric meta for Minecraft %s Fabric %s: %v", f.Minecraft.RawVersion, f.RawVersion, err)
//		}
//
//		f.metaCache = meta
//	}
//
//	return f.metaCache
//}

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

	if f.
	iFileName := fmt.Sprintf("fabric-server-mc.%s-loader.%s-launcher.%s.jar", f.Minecraft.RawVersion, f.RawVersion, f.InstallerCache[0].Version)
	iURL, err := url.Parse(fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/%s/%s/server/jar", f.Minecraft.RawVersion, f.RawVersion, f.InstallerCache[0].Version))
	if err != nil {
		log.Fatalf("Error parsing fabric installer download URL\n%v", err)
	}
	log.Printf("Installer download URL %s", iURL.String())
	downloads = append(downloads, Download{
		Path:     installPath,
		URL:      *iURL,
		Name:     iFileName,
		FullPath: filepath.Join(installPath, iFileName),
	})

	//meta := f.getMeta()
	//homeDir := getFabricHomeDir()
	//for _, library := range meta.Libraries {
	//	mavenURL, filename := getMavenUrl(library.Name)
	//	mavenURL = GetMirrorFor(mavenURL, library.URL)
	//	sha1URL := mavenURL + ".sha1"
	//	sha1 := getOrBlank(sha1URL)
	//
	//	parse, err := url.Parse(mavenURL)
	//	if err != nil {
	//		//shrug
	//		continue
	//	}
	//
	//	downloads = append(downloads, Download{filepath.Join(homeDir, ".cache"), *parse, filename, "sha1", sha1, filepath.Join(homeDir, ".cache", filename)})
	//}
	return downloads
}

func (f Fabric) Install(installPath string, java JavaProvider) bool {
	log.Println("Installing Fabric")
	//serverName := fmt.Sprintf("fabric-%s-%s-server-launch.jar", f.Minecraft.RawVersion, f.FabricVersion.RawVersion)
	//meta := f.getMeta()

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

	os.WriteFile(filepath.Join(installPath, "fabric-server-launcher.properties"), []byte("serverJar="+serverDownload.Name+"\n"), 0644)

	//mergeZips(jars, filepath.Join(installPath, serverName), false, meta.MainClass)

	return true
}

func (f Fabric) GetLaunchJar(installPath string) (string, []string) {
	return fmt.Sprintf("fabric-server-mc.%s-loader.%s-launcher.%s.jar", f.Minecraft.RawVersion, f.RawVersion, f.InstallerCache[0].Version), nil
}

func GetFabric(modloader Target, mc Minecraft) (error, ModLoader) {
	fab := Fabric{}
	fab.FabricVersion.RawVersion = modloader.Version
	fab.FabricVersion.Minecraft = mc
	fab.InstallerCache = getInstaller()
	return nil, fab
}

func getInstaller() FabricMetaInstaller {
	var url = FABRIC_META + "/v2/versions/installer"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("error getting fabric meta for Minecraft %s Fabric", err)
	}

	defer resp.Body.Close()
	var fInstallers FabricMetaInstaller
	err = json.NewDecoder(resp.Body).Decode(&fInstallers)
	if err != nil {
		log.Fatalf("error decoding response\n%v", err)
	}

	return fInstallers
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
