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
	hashVer "github.com/hashicorp/go-version"
)

func GetNeoForge(modloader Target, mc Minecraft) (error, ModLoader) {
	version := NeoForgeVersion{}
	version.RawVersion = modloader.Version
	version.Minecraft = mc
	err := version.Parse()
	if err != nil {
		return err, nil
	}
	targetMcVer, _ := hashVer.NewVersion(version.Minecraft.RawVersion)
	breakingVersion, _ := hashVer.NewVersion("1.20.2") // NeoForge changed maven/package names
	if targetMcVer.GreaterThanOrEqual(breakingVersion) {
		version.AfterBreaking = true
	}
	return nil, NeoForgeInstall{version}
}

type NeoForgeVersion struct {
	RawVersion    string
	Major         int
	Minor         int
	Build         int
	Minecraft     Minecraft
	AfterBreaking bool
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

const neoForgeUrlInstallJar = "https://maven.neoforged.net/releases/net/neoforged/%s/%s/%s"
const neoForgeUrlInstallJSON = "https://maven.neoforged.net/releases/net/neoforged/%s/%s/%s-%s.json"

func GetNeoMirrors() []string {
	return []string{"https://maven.neoforged.net/releases", "https://libraries.minecraft.net/"}
}

type NeoForgeInstall struct {
	Version NeoForgeVersion
}

func (f NeoForgeInstall) GetDownloads(installPath string) []Download {
	printfln("Getting downloads for NeoForge Install")
	var packageName string
	var versionStr string
	if f.Version.AfterBreaking {
		versionStr = f.Version.RawVersion
		packageName = "neoforge"
	} else {
		versionStr = fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)
		packageName = "forge"
	}
	installerName := fmt.Sprintf("%s-%s-installer.jar", packageName, versionStr)
	forgeUrl := fmt.Sprintf(neoForgeUrlInstallJar, packageName, versionStr, installerName)
	forgeUrlJSON := fmt.Sprintf(neoForgeUrlInstallJSON, packageName, versionStr, packageName, versionStr)
	var rawForgeJSON []byte
	var rawForgeInstallJSON []byte
	if !FileOnServer(forgeUrlJSON) {
		resp, err := grab.Get(installPath, forgeUrl)
		if err != nil {
			fatalf("JSON not on server and unable to get forge jar:\n%s\n%s\n %v", forgeUrlJSON, forgeUrl, err)
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
	printfln("Running NeoForge installer")
	var packageName string
	if f.Version.AfterBreaking {
		packageName = "neoforge"
	} else {
		packageName = "forge"
	}

	retryCount := 0
Forge:
	xmx := "2048M"
	var versionStr string
	if f.Version.AfterBreaking {
		versionStr = f.Version.RawVersion

	} else {
		versionStr = fmt.Sprintf(versionFmt, f.Version.Minecraft.RawVersion, f.Version.RawVersion)

	}
	installerName := fmt.Sprintf("%s-%s-installer.jar", packageName, versionStr)

	javaPath := ""
	if retryCount >= 2 {
		printfln("Install failed twice or more times, trying system Java")
		javaPath = "java"
	} else {
		javaPath = java.GetJavaPath("")
	}
	fmt.Println("Java Path has been set to:", javaPath)
	LogIfVerbose("Running %s -Xmx%s -jar %s --installServer", javaPath, xmx, installerName)
	cmd := exec.Command(javaPath, "-Xmx"+xmx, "-jar", installerName, "--installServer")
	cmd.Dir = installPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatalf(fmt.Sprintf("Running neoforge installer failed with %s. You may wish to install neoforge %s for Minecraft %s manually", err, f.Version.RawVersion, f.Version.Minecraft.RawVersion))
		return false
	}
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				printfln(fmt.Sprintf("NeoForge installer failed with exit code %d, retrying...", exitErr.ExitCode()))
				retryCount++
				if retryCount < 3 {
					goto Forge
				} else {
					fatalf("NeoForge failed to install multiple times exiting...")
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

	var packageName string
	if f.Version.AfterBreaking {
		packageName = "neoforge"
	} else {
		packageName = "forge"
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
		var modloaderVersion string
		if f.Version.AfterBreaking {
			modloaderVersion = forgeVer
		} else {
			modloaderVersion = mcVer + "-" + forgeVer
		}
		jvmArgs = append(jvmArgs, "@user_jvm_args.txt")
		jvmArgs = append(jvmArgs, "@"+filepath.Join("libraries", "net", "neoforged", packageName, modloaderVersion, argsTxt))

		return "", jvmArgs
	}
	return "insert-jar-here.jar", nil
}
