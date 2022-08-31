package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/cavaliercoder/grab"
)

func GetForge(modloader Target, mc Minecraft) (error, ModLoader) {
	version := ForgeVersion{}
	version.RawVersion = modloader.Version
	version.Minecraft = mc
	err := version.Parse()
	if err != nil {
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
	Major      int
	Minor      int
	Build      int
	Minecraft  Minecraft
}

func (f *ForgeVersion) Parse() error {
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

		val, err = strconv.Atoi(splitVer[len(splitVer)-1])
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
	versionStrOther := fmt.Sprintf(versionFmtOther, f.Version.Minecraft.RawVersion, f.Version.RawVersion, f.Version.Minecraft.RawVersion)
	universalName := fmt.Sprintf("forge-%s-universal.jar", versionStr)
	universalNameOther := fmt.Sprintf("forge-%s-universal.jar", versionStrOther)
	forgeUrl := fmt.Sprintf(forgeUrlUniversalJar, versionStr, universalName)
	forgeUrl = GetMirrorFor(forgeUrl, "https://maven.creeperhost.net/")
	forgeUrlOther := fmt.Sprintf(forgeUrlUniversalJar, versionStrOther, universalNameOther)
	forgeUrlJSON := fmt.Sprintf(forgeUrlInstallJSON, versionStr, versionStr)
	forgeUrlJSONOther := fmt.Sprintf(forgeUrlInstallJSON, versionStrOther, versionStrOther)
	var rawForgeJSON []byte
	if !FileOnServer(forgeUrlJSON) {
		forgeUrlJSON = forgeUrlJSONOther
	}
	if !FileOnServer(forgeUrl) {
		forgeUrl = forgeUrlOther
		universalName = universalNameOther
	}
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
	downloads := []Download{{"", *URL, universalName, "", "", path.Join("", universalName)}}

	if len(rawForgeJSON) > 0 {
		versionForge := VersionJson{}
		err := json.Unmarshal(rawForgeJSON, &versionForge)
		if err == nil {
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

func (f ForgeUniversal) GetLaunchJar(installPath string) (string, []string) {
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion, f.Version.Minecraft.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	return "insert-jar-here.jar", nil
}

const versionFmt = "%s-%s"
const versionFmtOther = "%s-%s-%s"

const forgeUrlUniversalJar = "https://maven.creeperhost.net/net/minecraftforge/forge/%s/%s"
const forgeUrlInstallJar = "https://maven.creeperhost.net/net/minecraftforge/forge/%s/%s"
const forgeUrlInstallJSON = "https://maven.creeperhost.net/net/minecraftforge/forge/%s/forge-%s.json"

func GetMirrors() []string {
	return []string{"https://maven.creeperhost.net/", "https://libraries.minecraft.net/"}
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
	var rawForgeInstallJSON []byte
	if !FileOnServer(forgeUrlJSON) {
		resp, err := grab.Get(installPath, forgeUrl)
		if err != nil {
			log.Fatalf("JSON not on server and unable to get forge jar: %v", err)
		}
		if resp.IsComplete() {
			resp.Wait()
		}
		bytes, err := UnzipFileToMemory(path.Join(installPath, installerName), "version.json")
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

	bytes, err := UnzipFileToMemory(path.Join(installPath, installerName), "install_profile.json")
	if err == nil {
		rawForgeInstallJSON = bytes
	}

	URL, err := url.Parse(forgeUrl)
	if err != nil {
		log.Fatalf("Unable to get forge jar as error parsing URL somehow: URL: %s, Error: %v", forgeUrl, err)
	}
	downloads := []Download{{"", *URL, installerName, "", "", path.Join("", installerName+".jar")}}

	if len(rawForgeJSON) > 0 {
		versionForge := VersionJsonFG3{}
		err := json.Unmarshal(rawForgeJSON, &versionForge)
		if err == nil {
			downloads = append(downloads, versionForge.GetDownloads()...)
		}
	}
	if len(rawForgeInstallJSON) > 0 {
		versionForge := VersionJsonFG3{}
		err := json.Unmarshal(rawForgeInstallJSON, &versionForge)
		if err == nil {
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

func (f ForgeInstall) GetLaunchJar(installPath string) (string, []string) {
	mcVer := f.Version.Minecraft.RawVersion
	forgeVer := f.Version.RawVersion
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", mcVer, forgeVer)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-universal.jar", mcVer, forgeVer)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}

	// Detect Modular forge from the 'user_jvm_args.txt' file.
	if _, err := os.Stat(path.Join(installPath, "user_jvm_args.txt")); !os.IsNotExist(err) {
		// _YEEEET_ OUTTA HERE WITH YO SHEEEET
		os.Remove(path.Join(installPath, "run.bat"))
		os.Remove(path.Join(installPath, "run.sh"))

		var argsTxt string
		switch runtime.GOOS {
		case "windows":
			argsTxt = "win_args.txt"
		default:
			argsTxt = "unix_args.txt"
		}

		var jvmArgs []string
		jvmArgs = append(jvmArgs, "@user_jvm_args.txt")
		jvmArgs = append(jvmArgs, "@"+path.Join("libraries", "net", "minecraftforge", "forge", mcVer+"-"+forgeVer, argsTxt))

		return "", jvmArgs
	}
	return "insert-jar-here.jar", nil
}

type ForgeInJar struct {
	Version ForgeVersion
}

type ourFileInfo struct {
	os.FileInfo
	directory string
}

type hashName struct {
	name string
	hash string
}

func (f ForgeInJar) GetDownloads(installPath string) []Download {
	log.Println("Getting downloads for Forge In Jar")
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	serverName := fmt.Sprintf("forge-%s-universal.zip", versionStr)
	if f.Version.Minecraft.RawVersion == "1.2.5" {
		serverName = fmt.Sprintf("forge-%s-server.zip", versionStr)
	}
	forgeUrl := fmt.Sprintf(forgeUrlUniversalJar, versionStr, serverName)
	forgeUrl = GetMirrorFor(forgeUrl, "https://maven.minecraftforge.net/")

	URL, err := url.Parse(forgeUrl)
	if err != nil {
		log.Fatalf("Unable to get forge jar as error parsing URL somehow: URL: %s, Error: %v", forgeUrl, err)
	}

	vanillaVer, err := f.Version.Minecraft.GetVanillaVersion()
	if err != nil {
		// handleerr
	}

	serverDownload, err := vanillaVer.GetServerDownload()
	if err != nil {
		// handleerr
	}

	libs := make(map[string]hashName)

	if f.Version.Minecraft.RawVersion == "1.4.7" {
		libs["https://maven.creeperhost.net/net/sourceforge/argo/argo/2.25/argo-2.25.jar"] = hashName{"argo-2.25.jar", "bb672829fde76cb163004752b86b0484bd0a7f4b"}
		libs["https://maven.creeperhost.net/com/google/guava/guava/12.0.1/guava-12.0.1.jar"] = hashName{"guava-12.0.1.jar", "b8e78b9af7bf45900e14c6f958486b6ca682195f"}
		libs["https://maven.creeperhost.net/org/ow2/asm/asm-all/4.0/asm-all-4.0-fml.jar"] = hashName{"asm-all-4.0.jar", "98308890597acb64047f7e896638e0d98753ae82"}
		libs["https://maven.creeperhost.net/org/bouncycastle/bcprov-jdk15on/1.47/bcprov-jdk15on-1.47.jar"] = hashName{"bcprov-jdk15on-147.jar", "b6f5d9926b0afbde9f4dbe3db88c5247be7794bb"}
	}

	if f.Version.Minecraft.RawVersion == "1.5.2" {
		libs["https://maven.creeperhost.net/net/sourceforge/argo/argo/3.2/argo-3.2-small.jar"] = hashName{"argo-small-3.2.jar", "58912ea2858d168c50781f956fa5b59f0f7c6b51"}
		libs["https://maven.creeperhost.net/com/google/guava/guava/14.0-rc3/guava-14.0-rc3.jar"] = hashName{"guava-14.0-rc3.jar", "931ae21fa8014c3ce686aaa621eae565fefb1a6a"}
		libs["https://maven.creeperhost.net/org/ow2/asm/asm-all/4.1/asm-all-4.1.jar"] = hashName{"asm-all-4.1.jar", "054986e962b88d8660ae4566475658469595ef58"}
		libs["https://maven.creeperhost.net/org/bouncycastle/bcprov-jdk15on/1.48/bcprov-jdk15on-1.48.jar"] = hashName{"bcprov-jdk15on-148.jar", "960dea7c9181ba0b17e8bab0c06a43f0a5f04e65"}
		libs["https://maven.creeperhost.net/cpw/mods/fml/deobfuscation_data/1.5.2/deobfuscation_data-1.5.2.zip"] = hashName{"deobfuscation_data_1.5.2.zip", "446e55cd986582c70fcf12cb27bc00114c5adfd9"}
		libs["https://maven.creeperhost.net/org/scala-lang/scala-library/2.10.0/scala-library-2.10.0.jar"] = hashName{"scala-library.jar", "458d046151ad179c85429ed7420ffb1eaf6ddf85"}
	}

	downloads := []Download{serverDownload, {"", *URL, serverName, "", "", path.Join("", serverName)}}

	for libUrl, lib := range libs {
		URL, err := url.Parse(libUrl)
		if err != nil {
			if err != nil {
				log.Fatalf("Couldn't download lib as error parsing URL somehow: URL: %s, Error: %v", libUrl, err)
			}
		}
		baseName := lib.name
		downloads = append(downloads, Download{"lib/", *URL, baseName, "sha1", lib.hash, path.Join("lib/", baseName)})
	}

	return downloads
}

func (f ForgeInJar) Install(installPath string) bool {
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	serverNameDownloaded := fmt.Sprintf("forge-%s-universal.zip", versionStr)
	if f.Version.Minecraft.RawVersion == "1.2.5" {
		serverNameDownloaded = fmt.Sprintf("forge-%s-server.zip", versionStr)
	}

	serverName := fmt.Sprintf("forge-%s-universal.jar", versionStr)

	directories := make([]string, 2)

	directories[0] = path.Join(installPath, "instmods")
	directories[1] = path.Join(installPath, "jarmods")

	jarMods := listDirectories(directories)

	vanillaVer, err := f.Version.Minecraft.GetVanillaVersion()
	if err != nil {
		// handleerr
	}

	serverDownload, err := vanillaVer.GetServerDownload()
	if err != nil {
		// handleerr
	}

	mergeJars := []string{path.Join(installPath, serverDownload.Path, serverDownload.Name), path.Join(installPath, serverNameDownloaded)}

	mergeJars = append(mergeJars, jarMods...)

	mergeZips(mergeJars, path.Join(installPath, serverName), true, "")

	return true
}

func (f ForgeInJar) GetLaunchJar(installPath string) (string, []string) {
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(path.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	return "insert-jar-here.jar", nil
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
		} // we've either already downloaded it or the forge installer will take care of it, not perfect but avoids errors
		actualUrl, err := url.Parse(artichoke.Url)
		if err != nil {
			continue
		}
		hash := ""
		if len(artichoke.Hashes) > 0 {
			hash = artichoke.Hashes[0]
		}
		downloads = append(downloads, Download{path.Join("libraries", dir), *actualUrl, file, "sha1", hash, path.Join("libraries", dir, file)})
	}
	return downloads
}

type VersionLibrary struct {
	Name     string   `json:"name"`
	Server   bool     `json:"serverreq"`
	BaseURL  string   `json:"url"`
	Hashes   []string `json:"hashes"`
	Url      string
	Path     string
	Filename string
}

func (v *VersionLibrary) UnmarshalJSON(data []byte) error {
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

	v.Url = GetMirrorFor(pathTemp+"/"+filename, "https://maven.minecraftforge.net/")
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
			downloads = append(downloads, Download{path.Join("libraries", dir), *actualUrl, file, "sha1", artichoke.SHA1, path.Join("libraries", dir, file)})
		}
	}
	return downloads
}

type VersionLibraryFG3 struct {
	Name     string `json:"name"`
	Download struct {
		Artifact struct {
			Path string `json:"path"`
			Url  string `json:"url"`
			SHA1 string `json:"sha1"`
		} `json:"artifact"`
	} `json:"downloads"`
}
