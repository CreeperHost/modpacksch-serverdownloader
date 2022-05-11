package main

import (
	"log"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
)

const ADOPTIUM_URL = "https://api.adoptium.net"

type JavaProvider interface {
	GetDownloads(installPath string) []Download
	Install(installPath string) bool
	GetJavaPath(installPath string) string
}

// region NoOp

type NoOpJavaProvider struct {
}

func (e *NoOpJavaProvider) GetDownloads(installPath string) []Download {
	return make([]Download, 0)
}

func (e *NoOpJavaProvider) Install(installPath string) bool {
	return true
}

func (e *NoOpJavaProvider) GetJavaPath(installPath string) string {
	switch runtime.GOOS {
	case "windows":
		return "java.exe"
	default:
		return "java"
	}
}

// endregion

//region Adoptium

type AdoptiumRelease struct {
	Binaries    []Binary    `json:"binaries"`
	ReleaseName string      `json:"release_name"`
	VersionData VersionData `json:"version_data"`
}

type Binary struct {
	ImageType string  `json:"image_type"`
	Package   Package `json:"package"`
}

type Package struct {
	Checksum string `json:"checksum"`
	Link     string `json:"link"`
	Name     string `json:"name"`
	Size     int    `json:"size"`
}

type VersionData struct {
	Semver string `json:"semver"`
}

type InstallProperties struct {
	Release     *AdoptiumRelease
	Binary      *Binary
	ArchivePath *string
}

type AdoptiumJavaProvider struct {
	ShortVersion *string
	SemverTarget *string
	InstallProps *InstallProperties
}

func (self *AdoptiumJavaProvider) GetDownloads(installPath string) []Download {
	downloads := make([]Download, 0)

	rel, err := self.GetCompatiableAdoptiumVersion()
	if err != nil {
		return downloads
	}

	// Must have binaries.
	if len(rel.Binaries) == 0 {
		return downloads
	}

	// Just grab the first
	binary := rel.Binaries[0]

	var ext string
	if strings.HasSuffix(binary.Package.Name, ".zip") {
		ext = ".zip"
	} else if strings.HasSuffix(binary.Package.Name, ".tar.gz") {
		ext = ".tar.gz"
	} else {
		return downloads
	}

	parsedUrl, err := url.Parse(binary.Package.Link)
	if err != nil {
		// ookay, we tried i guess?
		return downloads
	}

	jrePath := path.Join(installPath, "jre")
	archiveName := "jre" + ext
	fullPath := path.Join(jrePath, archiveName)

	version := InstallProperties{rel, &binary, &fullPath}
	self.InstallProps = &version

	downloads = append(downloads, Download{jrePath, *parsedUrl, archiveName, "sha256", binary.Package.Checksum, fullPath})

	return downloads
}

func (self *AdoptiumJavaProvider) Install(installPath string) bool {
	if self.InstallProps != nil {
		archivePath := *self.InstallProps.ArchivePath
		if strings.HasSuffix(archivePath, ".zip") {
			err := extractZip(path.Join(installPath, "jre"), archivePath)
			if err != nil {
				log.Println("Failed to extract zip: " + archivePath)
				log.Println(err)
				return false
			}
			os.Remove(archivePath)
		} else if strings.HasSuffix(archivePath, ".tar.gz") {
			err := extractTarGz(path.Join(installPath, "jre"), archivePath)
			if err != nil {
				log.Println("Failed to extract tar.gz: " + archivePath)
				log.Println(err)
				return false
			}
			os.Remove(archivePath)
		} else {
			log.Println("I don't know how to extract this adoptium archive: " + archivePath)
			return false
		}
	}
	return true
}

func (self *AdoptiumJavaProvider) GetJavaPath(installPath string) string {
	var executable = "java"
	var binFolder = "bin"
	switch runtime.GOOS {
	case "windows":
		executable = "java.exe"
		binFolder = "bin"
	case "freebsd":
		fallthrough
	case "linux":
		executable = "java"
		binFolder = "bin"
	case "darwin":
		executable = "java"
		binFolder = path.Join("Contents", "Home", "bin")
	}

	if self.InstallProps != nil {
		return path.Join(installPath,
			"jre",
			self.InstallProps.Release.ReleaseName+"-"+self.InstallProps.Binary.ImageType,
			binFolder,
			executable)
	}

	return executable
}

func (self *AdoptiumJavaProvider) GetCompatiableAdoptiumVersion() (*AdoptiumRelease, error) {
	if self.SemverTarget != nil {
		return self.GetAdoptiumReleaseViaSemver(runtime.GOARCH, true)
	} else {
		return self.GetLatestAdoptiumRelease(runtime.GOARCH, true)
	}
}

func (self *AdoptiumJavaProvider) GetAdoptiumReleaseViaSemver(architecture string, jre bool) (*AdoptiumRelease, error) {
	var releases []AdoptiumRelease
	url := ADOPTIUM_URL + "/v3/assets/version/"
	url += *self.SemverTarget
	url += GetAdoptiumQueryProperties(architecture, jre)
	err := APICall(url, &releases)
	if err != nil {
		if runtime.GOOS == "darwin" && architecture == "arm64" {
			// We are mac M1, try x64.
			return self.GetAdoptiumReleaseViaSemver("amd64", jre)
		}
		if jre {
			// We failed to find a JRE, find a JDK instead..
			return self.GetAdoptiumReleaseViaSemver(architecture, false)
		}
		return nil, err
	}

	return &releases[0], nil
}

func (self *AdoptiumJavaProvider) GetLatestAdoptiumRelease(architecture string, jre bool) (*AdoptiumRelease, error) {
	var releases []AdoptiumRelease
	url := ADOPTIUM_URL + "/v3/assets/feature_releases/"
	url += *self.ShortVersion
	url += "/ga"
	url += GetAdoptiumQueryProperties(architecture, jre)
	err := APICall(url, &releases)
	if err != nil {
		if runtime.GOOS == "darwin" && architecture == "arm64" {
			// We are mac M1, try x64.
			return self.GetLatestAdoptiumRelease("amd64", jre)
		}
		if jre {
			// We failed to find a JRE, find a JDK instead..
			return self.GetLatestAdoptiumRelease(architecture, false)
		}
		return nil, err
	}

	return &releases[0], nil
}

func GetAdoptiumQueryProperties(architecture string, jre bool) string {
	os := runtime.GOOS
	if os == "darwin" {
		os = "mac"
	}
	if architecture == "amd64" {
		architecture = "x64"
	} else if architecture == "386" {
		architecture = "x86"
	} else if architecture == "arm64" {
		architecture = "aarch64"
	}

	var imageType string
	if jre {
		imageType = "jre"
	} else {
		imageType = "jdk"
	}

	return "?project=jdk" +
		"&image_type=" + imageType +
		"&vendor=eclipse" +
		"&jvm_impl=hotspot" +
		"&heap_size=normal" +
		"&architecture=" + architecture +
		"&os=" + os
}

//endregion
