package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/cavaliergopher/grab/v3"
)

func GetNeoForge(modloader Target, mc Minecraft) (error, ModLoader) {
	version := NeoForgeVersion{}
	version.RawVersion = modloader.Version
	version.Minecraft = mc
	err := version.Parse()
	if err != nil {
		return err, nil
	}
	if mc.MinorVersion >= 13 || (mc.MinorVersion == 12 && version.Build >= 2851) {
		return nil, NeoForgeInstall{version}
	}
	if mc.MinorVersion > 5 {
		return nil, NeoForgeUniversal{version}
	}
	return nil, NeoForgeInJar{version}
}

type NeoForgeVersion struct {
	RawVersion string
	Major      int
	Minor      int
	Build      int
	Minecraft  Minecraft
}

func (f *NeoForgeVersion) Parse() error {
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

	return errors.New(fmt.Sprintf("neoforge Version does not match expected format: %s", f.RawVersion))
}

type NeoForgeUniversal struct {
	Version NeoForgeVersion
}

func (f NeoForgeUniversal) GetDownloads(installPath string) []Download {
	printfln("Getting downloads for NeoForged Universal")
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	versionStrOther := fmt.Sprintf(versionFmtOther, f.Version.Minecraft.RawVersion, f.Version.RawVersion, f.Version.Minecraft.RawVersion)
	universalName := fmt.Sprintf("forge-%s-universal.jar", versionStr)
	universalNameOther := fmt.Sprintf("forge-%s-universal.jar", versionStrOther)
	forgeUrl := fmt.Sprintf(neoForgeUrlUniversalJar, versionStr, universalName)
	forgeUrl = GetMirrorFor(forgeUrl, "https://apps.modpacks.ch/versions/")
	forgeUrlOther := fmt.Sprintf(neoForgeUrlUniversalJar, versionStrOther, universalNameOther)
	forgeUrlJSON := fmt.Sprintf(neoForgeUrlInstallJSON, versionStr, versionStr)
	forgeUrlJSONOther := fmt.Sprintf(neoForgeUrlInstallJSON, versionStrOther, versionStrOther)
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
			fatalf("JSON not on server and unable to get neoforged jar: %v", err)
		}
		if resp.IsComplete() {
			resp.Wait()
		}
		bytes, err := UnzipFileToMemory(filepath.Join(installPath, universalName), "version.json")
		if err == nil {
			rawForgeJSON = bytes
		}
	} else {
		resp, err := http.Get(forgeUrlJSON)
		if err == nil {
			defer resp.Body.Close()
			bytes, err := io.ReadAll(resp.Body)
			if err == nil {
				rawForgeJSON = bytes
			}
		}
	}

	URL, err := url.Parse(forgeUrl)
	if err != nil {
		fatalf("Unable to get neoforged jar as error parsing URL somehow: URL: %s, Error: %v", forgeUrl, err)
	}
	downloads := []Download{{"", *URL, universalName, "", "", filepath.Join("", universalName)}}

	if len(rawForgeJSON) > 0 {
		versionForge := VersionJson{}
		err := json.Unmarshal(rawForgeJSON, &versionForge)
		if err == nil {
			downloads = append(downloads, versionForge.GetLibraryDownloads()...)
		} else {
			fatalf("Cannot get a json to download the libraries which is required: %v", err)
		}
	} else {
		fatalf("Cannot get a json to download the libraries which is required.")
	}
	vanillaVer, err := f.Version.Minecraft.GetVanillaVersion()
	if err == nil {
		serverDownload, err := vanillaVer.GetServerDownload()
		if err == nil {
			downloads = append(downloads, serverDownload)
		}
	}
	if err != nil {
		printf("Unable to get Minecraft server jar - but neoforged will try again anyway. Error: %v", err)
	}
	return downloads
}

func (f NeoForgeUniversal) Install(installPath string, java JavaProvider) bool {
	return true
}

func (f NeoForgeUniversal) GetLaunchJar(installPath string) (string, []string) {
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(filepath.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(filepath.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion, f.Version.Minecraft.RawVersion)
	if _, err := os.Stat(filepath.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	return "insert-jar-here.jar", nil
}

const neoForgeUrlUniversalJar = "https://maven.neoforged.net/releases/net/neoforged/forge/%s/%s"
const neoForgeUrlInstallJar = "https://maven.neoforged.net/releases/net/neoforged/forge/%s/%s"
const neoForgeUrlInstallJSON = "https://maven.neoforged.net/releases/net/neoforged/forge/%s/forge-%s.json"

func GetNeoMirrors() []string {
	return []string{"https://maven.neoforged.net/releases", "https://libraries.minecraft.net/"}
}

type NeoForgeInstall struct {
	Version NeoForgeVersion
}

func (f NeoForgeInstall) GetDownloads(installPath string) []Download {
	printfln("Getting downloads for Forge Install")
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	installerName := fmt.Sprintf("forge-%s-installer.jar", versionStr)
	forgeUrl := fmt.Sprintf(neoForgeUrlInstallJar, versionStr, installerName)
	forgeUrlJSON := fmt.Sprintf(neoForgeUrlInstallJSON, versionStr, versionStr)
	var rawForgeJSON []byte
	var rawForgeInstallJSON []byte
	if !FileOnServer(forgeUrlJSON) {
		resp, err := grab.Get(installPath, forgeUrl)
		if err != nil {
			fatalf("JSON not on server and unable to get forge jar: %v", err)
		}
		if resp.IsComplete() {
			resp.Wait()
		}
		bytes, err := UnzipFileToMemory(filepath.Join(installPath, installerName), "version.json")
		if err == nil {
			rawForgeJSON = bytes
		}
	} else {
		resp, err := http.Get(forgeUrlJSON)
		if err == nil {
			defer resp.Body.Close()
			bytes, err := io.ReadAll(resp.Body)
			if err == nil {
				rawForgeJSON = bytes
			}
		}
	}

	bytes, err := UnzipFileToMemory(filepath.Join(installPath, installerName), "install_profile.json")
	if err == nil {
		rawForgeInstallJSON = bytes
	}

	URL, err := url.Parse(forgeUrl)
	if err != nil {
		fatalf("Unable to get forge jar as error parsing URL somehow: URL: %s, Error: %v", forgeUrl, err)
	}
	downloads := []Download{{"", *URL, installerName, "", "", filepath.Join("", installerName+".jar")}}

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
		printf("Unable to get Minecraft server jar - but forge will try again anyway. Error: %v", err)
	}
	return downloads
}

func (f NeoForgeInstall) Install(installPath string, java JavaProvider) bool {
	printfln("Running Forge installer")
	retryCount := 0
Forge:
	xmx := "2048M"
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	installerName := fmt.Sprintf("forge-%s-installer.jar", versionStr)
	javaPath := java.GetJavaPath("")
	if retryCount >= 2 {
		printfln("Install failed twice or more times, trying system Java")
		javaPath = "java"
	}
	LogIfVerbose("Running %s -Xmx%s -jar %s --installServer", javaPath, xmx, installerName)
	cmd := exec.Command(javaPath, "-Xmx"+xmx, "-jar", installerName, "--installServer")
	cmd.Dir = installPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatalf(fmt.Sprintf("Running forge installer failed with %s. You may wish to install forge %s for Minecraft %s manually", err, f.Version.RawVersion, f.Version.Minecraft.RawVersion))
		return false
	}
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				printfln(fmt.Sprintf("Forge installer failed with exit code %d, retrying...", exitErr.ExitCode()))
				retryCount++
				if retryCount < 3 {
					goto Forge
				} else {
					fatalf("Forge failed to install multiple times exiting...")
					os.Exit(1)
				}
			}
		} else {
			fatalf("cmd.Wait: %v", err)
		}
	}
	_ = os.Remove(filepath.Join(installPath, installerName) + ".log")
	_ = os.Remove(filepath.Join(installPath, installerName))
	return true
}

func (f NeoForgeInstall) GetLaunchJar(installPath string) (string, []string) {
	mcVer := f.Version.Minecraft.RawVersion
	forgeVer := f.Version.RawVersion
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", mcVer, forgeVer)
	if _, err := os.Stat(filepath.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-universal.jar", mcVer, forgeVer)
	if _, err := os.Stat(filepath.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}

	// Detect Modular forge from the 'user_jvm_args.txt' file.
	if _, err := os.Stat(filepath.Join(installPath, "user_jvm_args.txt")); !os.IsNotExist(err) {
		// _YEEEET_ OUTTA HERE WITH YO SHEEEET
		os.Remove(filepath.Join(installPath, "run.bat"))
		os.Remove(filepath.Join(installPath, "run.sh"))

		var argsTxt string
		switch runtime.GOOS {
		case "windows":
			argsTxt = "win_args.txt"
		default:
			argsTxt = "unix_args.txt"
		}

		var jvmArgs []string
		jvmArgs = append(jvmArgs, "@user_jvm_args.txt")
		jvmArgs = append(jvmArgs, "@"+filepath.Join("libraries", "net", "neoforged", "forge", mcVer+"-"+forgeVer, argsTxt))

		return "", jvmArgs
	}
	return "insert-jar-here.jar", nil
}

type NeoForgeInJar struct {
	Version NeoForgeVersion
}

func (f NeoForgeInJar) GetDownloads(installPath string) []Download {
	printfln("Getting downloads for Forge In Jar")
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	serverName := fmt.Sprintf("forge-%s-universal.zip", versionStr)
	if f.Version.Minecraft.RawVersion == "1.2.5" {
		serverName = fmt.Sprintf("forge-%s-server.zip", versionStr)
	}
	forgeUrl := fmt.Sprintf(neoForgeUrlUniversalJar, versionStr, serverName)
	forgeUrl = GetMirrorFor(forgeUrl, "https://maven.neoforged.net/releases")

	URL, err := url.Parse(forgeUrl)
	if err != nil {
		fatalf("Unable to get neoforged jar as error parsing URL somehow: URL: %s, Error: %v", forgeUrl, err)
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

	downloads := []Download{serverDownload, {"", *URL, serverName, "", "", filepath.Join("", serverName)}}

	for libUrl, lib := range libs {
		URL, err := url.Parse(libUrl)
		if err != nil {
			if err != nil {
				fatalf("Couldn't download lib as error parsing URL somehow: URL: %s, Error: %v", libUrl, err)
			}
		}
		baseName := lib.name
		downloads = append(downloads, Download{"lib/", *URL, baseName, "sha1", lib.hash, filepath.Join("lib/", baseName)})
	}

	return downloads
}

func (f NeoForgeInJar) Install(installPath string, java JavaProvider) bool {
	versionStr := fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	serverNameDownloaded := fmt.Sprintf("forge-%s-universal.zip", versionStr)
	if f.Version.Minecraft.RawVersion == "1.2.5" {
		serverNameDownloaded = fmt.Sprintf("forge-%s-server.zip", versionStr)
	}

	serverName := fmt.Sprintf("forge-%s-universal.jar", versionStr)

	directories := make([]string, 2)

	directories[0] = filepath.Join(installPath, "instmods")
	directories[1] = filepath.Join(installPath, "jarmods")

	jarMods := listDirectories(directories)

	vanillaVer, err := f.Version.Minecraft.GetVanillaVersion()
	if err != nil {
		// handleerr
	}

	serverDownload, err := vanillaVer.GetServerDownload()
	if err != nil {
		// handleerr
	}

	mergeJars := []string{filepath.Join(installPath, serverDownload.Path, serverDownload.Name), filepath.Join(installPath, serverNameDownloaded)}

	mergeJars = append(mergeJars, jarMods...)

	mergeZips(mergeJars, filepath.Join(installPath, serverName), true, "")

	return true
}

func (f NeoForgeInJar) GetLaunchJar(installPath string) (string, []string) {
	forgeJar := fmt.Sprintf("forge-%s-%s.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(filepath.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	forgeJar = fmt.Sprintf("forge-%s-%s-universal.jar", f.Version.Minecraft.RawVersion, f.Version.RawVersion)
	if _, err := os.Stat(filepath.Join(installPath, forgeJar)); err == nil {
		return forgeJar, nil
	}
	return "insert-jar-here.jar", nil
}
