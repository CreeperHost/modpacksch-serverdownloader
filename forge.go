package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cavaliercoder/grab"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

func GetForge(modloader Target, mc Minecraft) (error, ModLoader) {
	version := ForgeVersion{}
	version.RawVersion = modloader.Version
	version.Minecraft = mc
	err := version.Parse(); if err != nil {
		return err, nil
	}
	if mc.MinorVersion >= 13 || (mc.MinorVersion == 12 && version.Build >= 2851) {
		return nil, ForgeInstall{version}
	}
	if mc.MinorVersion > 5 {
		return nil, ForgeUniversal{version}
	}
	return nil, ForgeInJar{version}
}

type ForgeVersion struct {
	RawVersion string
	Major int
	Minor int
	Build int
	Minecraft Minecraft
}

func (f* ForgeVersion) Parse() error {
	splitVer := strings.Split(f.RawVersion, ".")

	if len(splitVer) >= 3 {
		val, err := strconv.Atoi(splitVer[0])
		if err != nil {
			return err
		}
		f.Major = val

		parsedStr := ""

		if len(splitVer) == 4 {
			if len(splitVer[1]) == 1 {
				parsedStr += "0"
			}
			parsedStr += splitVer[1]
			if len(splitVer[2]) == 1 {
				parsedStr += "0"
			}
			parsedStr += splitVer[2]
		} else {
			parsedStr = splitVer[1]
		}

		val, err = strconv.Atoi(parsedStr)
		if err != nil {
			return err
		}
		f.Minor = val

		val, err = strconv.Atoi(splitVer[len(splitVer) - 1])
		if err != nil {
			return err
		}
		f.Build = val

		return nil
	}

	return errors.New(fmt.Sprintf("forge Version does not match expected format: %s", f.RawVersion))
}

type ForgeUniversal struct {
	Version ForgeVersion
}

func (f ForgeUniversal) GetDownloads(installPath string) []Download {
	log.Println("Getting downloads for Forge Universal")
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	universalName := fmt.Sprintf("forge-%s-universal.jar", versionStr)
	forgeUrl := fmt.Sprintf(forgeUrlUniversalJar, versionStr, universalName)
	forgeUrlJSON := fmt.Sprintf(forgeUrlInstallJSON, versionStr, versionStr)
	var rawForgeJSON []byte
	if !FileOnServer(forgeUrlJSON) {
		resp, err := grab.Get(installPath, forgeUrl)
		if err != nil {
			log.Fatalf("JSON not on server and unable to get forge jar: %v", err)
		}
		if resp.IsComplete() {
			resp.Wait()
		}
		bytes, err := UnzipFileToMemory(path.Join(installPath, universalName), "version.json")
		if err == nil {
			rawForgeJSON = bytes
		}
	} else {
		resp, err := http.Get(forgeUrlJSON)
		if err == nil {
			defer resp.Body.Close()
			bytes, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				rawForgeJSON = bytes
			}
		}
	}

	URL, err := url.Parse(forgeUrl)
	if err != nil {
		log.Fatalf("Unable to get forge jar as error parsing URL somehow: URL: %s, Error: %v", forgeUrl, err)
	}
	downloads := []Download{{"", *URL, universalName, "", path.Join("", universalName)}}

	if len(rawForgeJSON) > 0 {
		versionForge := VersionJson{}
		err := json.Unmarshal(rawForgeJSON, &versionForge); if err == nil {
			downloads = append(downloads, versionForge.GetLibraryDownloads()...)
		} else {
			log.Fatalf("Cannot get a json to download the libraries which is required: %v", err)
		}
	} else {
		log.Fatalf("Cannot get a json to download the libraries which is required.")
	}
	vanillaVer, err := f.Version.Minecraft.GetVanillaVersion()
	if err == nil {
		serverDownload, err := vanillaVer.GetServerDownload()
		if err == nil {
			downloads = append(downloads, serverDownload)
		}
	}
	if err != nil {
		log.Printf("Unable to get Minecraft server jar - but forge will try again anyway. Error: %v", err)
	}
	return downloads
}

func (f ForgeUniversal) Install(installPath string) bool {
	return true
}

func (f ForgeUniversal) GetLaunchJar(installPath string) string {
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err != nil {
		return forgeJar
	}
	forgeJar =  fmt.Sprintf("forge-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err != nil {
		return forgeJar
	}
	return "insert-jar-here.jar"
}

const versionFmt = "%s-%s"
const versionFmtOther = "%s-%s"
//const forgeUrlMergeJar
const universalNameFmt = "forge-%s-%s-%s-universal"
const forgeUrlUniversalJar = "https://apps.modpacks.ch/versions/net/minecraftforge/forge/%s/%s"
const forgeUrlInstallJar = "https://apps.modpacks.ch/versions/net/minecraftforge/forge/%s/%s"
const forgeUrlInstallJSON = "https://apps.modpacks.ch/versions/net/minecraftforge/forge/%s/forge-%s.json"

func GetMirrors() []string {
	return []string{"https://libraries.minecraft.net/", "https://apps.modpacks.ch/versions/", "https://forge.modpacks.ch/maven/", "https://files.minecraftforge.net/maven/"}
}

func GetMirrorFor(urlStr string) string {
	mirrors := GetMirrors()
	var actualUrlStr string
	baseUrlStr := strings.Replace(urlStr, "https://files.minecraftforge.net/maven/", "", 1)
Out:
	for _, mirror := range mirrors {
		actualUrlStr = mirror + baseUrlStr
		if FileOnServer(actualUrlStr) {
			break Out
		}
	}
	return actualUrlStr
}

type ForgeInstall struct {
	Version ForgeVersion
}

func (f ForgeInstall) GetDownloads(installPath string) []Download {
	log.Println("Getting downloads for Forge Install")
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	installerName := fmt.Sprintf("forge-%s-installer.jar", versionStr)
	forgeUrl := fmt.Sprintf(forgeUrlInstallJar, versionStr, installerName)
	forgeUrlJSON := fmt.Sprintf(forgeUrlInstallJSON, versionStr, versionStr)
	var rawForgeJSON []byte
	if !FileOnServer(forgeUrlJSON) {
		resp, err := grab.Get(installPath, forgeUrl)
		if err != nil {
			log.Fatalf("JSON not on server and unable to get forge jar: %v", err)
		}
		if resp.IsComplete() {
			resp.Wait()
		}
		bytes, err := UnzipFileToMemory(path.Join(installPath, installerName + ".jar"), "version.json")
		if err == nil {
			rawForgeJSON = bytes
		} else {
			return []Download{} // Unable to get other downloads. Womp womp. Will let installer do it.
		}
	} else {
		resp, err := http.Get(forgeUrlJSON)
		if err == nil {
			defer resp.Body.Close()
			bytes, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				rawForgeJSON = bytes
			}
		}
	}

	URL, err := url.Parse(forgeUrl)
	if err != nil {
		log.Fatalf("Unable to get forge jar as error parsing URL somehow: URL: %s, Error: %v", forgeUrl, err)
	}
	downloads := []Download{{"", *URL, installerName + ".jar", "", path.Join("", installerName + ".jar")}}

	if len(rawForgeJSON) > 0 {
		versionForge := VersionJsonFG3{}
		err := json.Unmarshal(rawForgeJSON, &versionForge); if err == nil {
			downloads = append(downloads, versionForge.GetDownloads()...)
		}
	}
	vanillaVer, err := f.Version.Minecraft.GetVanillaVersion()
	if err == nil {
		serverDownload, err := vanillaVer.GetServerDownload()
		if err == nil {
			downloads = append(downloads, serverDownload)
		}
	}
	if err != nil {
		log.Printf("Unable to get Minecraft server jar - but forge will try again anyway. Error: %v", err)
	}
	return downloads
}

func (f ForgeInstall) Install(installPath string) bool {
	log.Println("Running Forge installer")
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	installerName := fmt.Sprintf("forge-%s-installer.jar", versionStr)
	LogIfVerbose("Running java -jar %s --installServer", installerName)
	cmd := exec.Command("java", "-jar", installerName, "--installServer")
	cmd.Dir = installPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Println(fmt.Sprintf("Running forge installer failed with %s. You may wish to install forge %s for Minecraft %s manually", err, f.Version.RawVersion, f.Version.Minecraft.RawVersion))
		return false
	}
	_ = os.Remove(path.Join(installPath, installerName) + ".log")
	_ = os.Remove(path.Join(installPath, installerName))
	return true
}

func (f ForgeInstall) GetLaunchJar(installPath string) string {
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err != nil {
		return forgeJar
	}
	forgeJar =  fmt.Sprintf("forge-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err != nil {
		return forgeJar
	}
	return "insert-jar-here.jar"
}

type ForgeInJar struct {
	Version ForgeVersion
}

func (f ForgeInJar) GetDownloads(installPath string) []Download {
	log.Println("Getting downloads for Forge In Jar")
	panic("implement me")
}

func (f ForgeInJar) Install(installPath string) bool {
	panic("implement me")
}

func (f ForgeInJar) GetLaunchJar(installPath string) string {
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar
	}
	forgeJar =  fmt.Sprintf("forge-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar
	}
	return "insert-jar-here.jar"
}

type VersionJson struct {
	Libraries []VersionLibrary
}

func (v VersionJson) GetLibraryDownloads() []Download {
	var downloads []Download
	for _, library := range v.Libraries {
		artichoke := library
		dir, file := artichoke.Path, artichoke.Filename
		if artichoke.Url == "" {
			continue
		}
		if strings.Contains(artichoke.Filename, "forge-") {
			continue
		}
		actualUrl, err := url.Parse(artichoke.Url)
		if err != nil {
			continue
		}
		hash := ""
		if len(artichoke.Hashes) > 0 {
			hash = artichoke.Hashes[0]
		}
		downloads = append(downloads, Download{path.Join("libraries", dir), *actualUrl, file, hash, path.Join("libraries", dir, file)})
	}
	return downloads
}

type VersionLibrary struct {
	Name     string `json:"name"`
	Server   bool `json:"serverreq"`
	BaseURL  string `json:"url"`
	Hashes   []string `json:"hashes"`
	Url      string
	Path     string
	Filename string
}

func (v* VersionLibrary) UnmarshalJSON(data []byte) error {
	type versionlibrary2 VersionLibrary
	if err := json.Unmarshal(data, (*versionlibrary2)(v)); err != nil {
		return err
	}
	split := strings.Split(v.Name, ":")
	v.Url = ""
	if split[1] == "minecraftforge" {
		return nil
	}
	filename := split[1] + "-" + split[2] + ".jar"
	pathTemp := strings.Replace(split[0], ".", "/", -1) + "/" + split[1] + "/" + split[2]

	v.Url = GetMirrorFor(pathTemp + "/" + filename)
	v.Filename = filename
	v.Path = pathTemp
	return nil
}

type VersionJsonFG3 struct {
	Libraries []VersionLibraryFG3
}

func (v VersionJsonFG3) GetDownloads() []Download {
	mirrors := GetMirrors()
	var downloads []Download
	for _, library := range v.Libraries {
		artichoke := library.Download.Artifact
		if len(artichoke.Url) > 0 {
			dir, file := path.Split(artichoke.Path)
			actualUrlStr := ""
		Out:
			for _, mirror := range mirrors {
				actualUrlStr = strings.Replace(artichoke.Url, "https://files.minecraftforge.net/maven/", mirror, 1)
				if FileOnServer(actualUrlStr) {
					break Out
				}
			}
			actualUrl, err := url.Parse(actualUrlStr)
			if err != nil {
				continue
			}
			downloads = append(downloads, Download{path.Join("libraries", dir), *actualUrl, file, artichoke.SHA1, path.Join("libraries", dir, file)})
		}
	}
	return downloads
}

type VersionLibraryFG3 struct {
	Name string `json:"name"`
	Download struct {
		Artifact struct {
			Path string `json:"path"`
			Url string  `json:"url"`
			SHA1 string `json:"sha1"`
		} `json:"artifact"`
	} `json:"downloads"`
}
