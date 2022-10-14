package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const MinecraftMetaURL = "https://launchermeta.mojang.com/mc/game/version_manifest.json"

func QuestionYN(def bool, question string, fmtArgs ...interface{}) bool {
	var defString = ""
	if def {
		defString = "y"
	} else {
		defString = "n"
	}

	choices := []string{"y", "n"}

	response := Question(defString, choices, true, question, fmtArgs...)

	if response == "y" {
		return true
	} else {
		return false
	}
}

func QuestionFree(def string, question string, fmtArgs ...interface{}) string {
	return Question(def, nil, false, question, fmtArgs...)
}

func Question(def string, choices []string, fixed bool, s string, fmtArgs ...interface{}) string {
	if Options.Auto == true {
		return def
	}

	choicesFmt := ""
	var choicesInt []interface{}

	if choices != nil {
		choicesFmt = "(" + strings.Trim(strings.Repeat("%s/", len(choices)), "/") + ")" + fmt.Sprintf(" [%v]", def)
		choicesInt = make([]interface{}, len(choices))
		for i := range choices {
			choicesInt[i] = choices[i]
		}
	} else {
		choicesInt = make([]interface{}, 0)
		choicesFmt = fmt.Sprintf("[%v]", def)
	}

	fmt.Println(fmt.Sprintf(s, fmtArgs...) + fmt.Sprintf(" "+choicesFmt, choicesInt...))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := scanner.Text()
	if len(response) == 0 {
		return def
	}
	if fixed {
		found := false
	Free:
		for i := range choices {
			if choices[i] == response {
				found = true
				break Free
			}
		}
		if found {
			return response
		}
		fmt.Println(fmt.Sprintf("\"%s\" is not a valid option.", response))
		return Question(def, choices, fixed, s, fmtArgs...)
	}
	return response
}

func (m *Minecraft) Parse() error {
	splitVer := strings.Split(m.RawVersion, ".")
	if len(splitVer) == 0 {
		return errors.New(fmt.Sprintf("minecraft Version does not match expected format: %s", m.RawVersion))
	}

	if len(splitVer) == 3 {
		val, err := strconv.Atoi(splitVer[2])
		if err != nil {
			return err
		}
		m.FixesVersion = val
	}

	val, err := strconv.Atoi(splitVer[0])
	if err != nil {
		return err
	}
	m.MajorVersion = val

	val, err = strconv.Atoi(splitVer[1])
	if err != nil {
		return err
	}
	m.MinorVersion = val

	return nil
}

func UnzipFileToMemory(archive string, filePath string) ([]byte, error) {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var file *zip.File

	for _, tempFile := range reader.File {
		if tempFile.Name == filePath {
			file = tempFile
			break
		}
	}

	if file == nil {
		return nil, errors.New(fmt.Sprintf("file %s not found in archive %s", filePath, archive))
	}

	fileReader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()

	bytes, err := io.ReadAll(fileReader)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func FileOnServer(urlPath string) bool {
	resp, err := http.Head(urlPath)
	return err == nil && resp.StatusCode == 200
}

type Minecraft struct {
	RawVersion   string
	MajorVersion int
	MinorVersion int
	FixesVersion int
}

type VanillaListManifest struct {
	Versions []VanillaVersion `json:"versions"`
}

type VanillaVersion struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type VanillaManifest struct {
	Downloads struct {
		Server struct {
			SHA1 string
			URL  string
		} `json:"server"`
	} `json:"downloads"`
}

func (m Minecraft) GetVanillaVersion() (VanillaVersion, error) {
	var ret VanillaVersion
	resp, err := http.Get(MinecraftMetaURL)
	if err == nil {
		defer resp.Body.Close()
		bytes, err := io.ReadAll(resp.Body)
		if err == nil {
			var manifest VanillaListManifest
			err := json.Unmarshal(bytes, &manifest)
			if err == nil {
				for _, v := range manifest.Versions {
					if v.ID == m.RawVersion {
						ret = v
						break
					}
				}
			}
		}
	}
	return ret, err
}

func (v VanillaVersion) GetServerDownload() (Download, error) {
	var ret Download
	resp, err := http.Get(v.URL)
	if err == nil {
		defer resp.Body.Close()
		bytes, err := io.ReadAll(resp.Body)
		if err == nil {
			vanillaManifest := VanillaManifest{}
			err = json.Unmarshal(bytes, &vanillaManifest)
			if err == nil {
				URL, err := url.Parse(vanillaManifest.Downloads.Server.URL)
				if err == nil {
					ret = Download{"", *URL, "minecraft_server." + v.ID + ".jar", "sha1", vanillaManifest.Downloads.Server.SHA1, path.Join("", "minecraft_server."+v.ID+".jar")}
				}
			}
		}
	}
	return ret, err
}

func mergeZips(zips []string, destzip string, deleteAfter bool, mainClass string) {
	zipfile, err := os.Create(destzip)
	if err != nil {
		log.Printf("Error opening %s for writing - running server may not work properly: %v\n", destzip, err)
	}
	buf := bufio.NewWriter(zipfile)
	w := zip.NewWriter(buf)

	reverseAny(zips)

	storedFiles := make(map[string]bool)

	services := make(map[string][]string)

	for _, file := range zips {
		r, err := zip.OpenReader(file)
		if err != nil {
			log.Printf("Error opening %s to merge into %s\n", file, destzip)
			continue
		}

		for _, f := range r.File {
			if strings.HasPrefix(f.Name, "META-INF/services/") && !strings.HasSuffix(f.Name, "/") {
				rc, err := f.Open()
				if err != nil {
					log.Printf("Error reading %s from %s to add services definition:%v\n", f.Name, file, err)
					continue
				}

				scanner := bufio.NewScanner(rc)

				for scanner.Scan() {
					line := scanner.Text()
					pos := strings.IndexByte(line, '#')
					if pos >= 0 {
						line = line[:pos]
					}

					line = strings.TrimSpace(line)

					if len(line) > 0 {
						prev, ok := services[f.Name]
						var arr []string
						if ok {
							arr = prev
						} else {
							arr = make([]string, 0)
						}

						services[f.Name] = append(arr, line)
					}
				}

				rc.Close()
			} else if !storedFiles[f.Name] && !(len(mainClass) > 0 && f.Name == "META-INF/MANIFEST.MF") {
				storedFiles[f.Name] = true
				rc, err := f.Open()
				if err != nil {
					log.Printf("Error writing %s from %s to %s:%v\n", f.Name, file, destzip, err)
					continue
				}
				wc, _ := w.Create(f.Name)

				_, err = io.Copy(wc, rc)

				rc.Close()

				if err != nil {
					log.Printf("Error writing %s from %s to %s:%v\n", f.Name, file, destzip, err)
				}
			}
		}

		r.Close()

		if deleteAfter {
			os.Remove(file)

			dir := path.Dir(file)
			files := listDirectories([]string{dir})
			if len(files) == 0 {
				os.Remove(dir)
			}
		}
	}

	for serviceName, lines := range services {
		fileContents := ""
		for _, line := range lines {
			fileContents += line + "\n"
		}

		wc, _ := w.Create(serviceName)

		wc.Write([]byte(fileContents))
	}

	if len(mainClass) != 0 {
		wc, _ := w.Create("META-INF/MANIFEST.MF")
		fileContents :=
			"Manifest-Version: 1.0\n" +
				"Main-Class: net.fabricmc.loader.launch.server.FabricServerLauncher\n"

		wc.Write([]byte(fileContents))

		wc, _ = w.Create("fabric-server-launch.properties")

		wc.Write([]byte("launch.mainClass=" + mainClass + "\n"))
	}

	w.Close()
	zipfile.Close()

}

func reverseAny(s interface{}) {
	n := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func listDirectories(directories []string) []string {
	var directoryReturn []string
	var allFiles []ourFileInfo
	for _, dirname := range directories {
		f, err := os.Open(dirname)
		if err != nil {
			continue
		}
		list, err := f.Readdir(-1)
		f.Close()
		if err != nil {
			continue
		}
		for _, file := range list {
			allFiles = append(allFiles, ourFileInfo{file, dirname})
		}
	}

	sort.Slice(allFiles, func(i, j int) bool { return allFiles[i].Name() < allFiles[j].Name() })

	directoryReturn = make([]string, len(allFiles))

	for i, file := range allFiles {
		directoryReturn[i] = path.Join(file.directory, file.Name())
	}

	return directoryReturn
}

func getOrBlank(URL string) string {
	resp, err := http.Get(URL)
	if err != nil {
		return ""
	}

	defer resp.Body.Close()
	bytesRead, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(bytesRead)
}

func GetMirrorFor(urlStr string, fallback string) string {
	mirrors := GetMirrors()
	baseUrlStr := strings.Replace(urlStr, "https://files.minecraftforge.net/maven/", "", 1)
	baseUrlStr = strings.Replace(baseUrlStr, "https://apps.modpacks.ch/versions/", "", 1)
	baseUrlStr = strings.Replace(baseUrlStr, "https://maven.minecraftforge.net/", "", 1)
	baseUrlStr = strings.Replace(baseUrlStr, "https://maven.creeperhost.net/", "", 1)
	fullUrl := fallback + baseUrlStr
	for _, mirror := range mirrors {
		newStr := mirror + baseUrlStr
		if FileOnServer(newStr) {
			fullUrl = newStr
			break
		}
	}
	return fullUrl
}

func getKey() string {
	path, err := os.Executable()

	foundStr := "public"

	if err == nil {
		newPath, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = newPath
		}

		file, err := os.Open(path)
		if err == nil {
			stat, err := file.Stat()
			if err == nil {
				byteRead := make([]byte, 86)
				bytesRead, _ := file.ReadAt(byteRead, stat.Size()-86)
			main:
				for i := 0; i < bytesRead; i++ {
					if byteRead[i] == markerBytes[0] {
						lastIndex := 0
						for markerIndex := 1; markerIndex < len(markerBytes); markerIndex++ {
							if byteRead[i+markerIndex] != markerBytes[markerIndex] {
								continue main
							}
							lastIndex = i + markerIndex
						}
						foundStr = string(byteRead[lastIndex+1:])
						foundStr = strings.TrimSpace(foundStr)
						log.Println("Found private key, using instead of public")
					}
				}
			}
		}
	}
	return foundStr
}

func extractZip(dest string, zipPath string) error {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}

	defer archive.Close()

	for _, f := range archive.File {
		destPath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			err := os.MkdirAll(destPath, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}

		log.Printf("Extracting %s -> %s", f.Name, destPath)

		if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	return nil
}

func extractTarGz(dest string, zipPath string) error {
	file, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	uncompressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(uncompressed)

	for true {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			log.Printf("Extracting %s -> %s", header.Name, destPath)
			dstFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(dstFile, tarReader); err != nil {
				return err
			}
			dstFile.Close()
		case tar.TypeSymlink:
			// Ignored.
		default:
			log.Printf("Unhandled type: %d %s", header.Typeflag, header.Name)
		}
	}
	file.Close()
	return nil
}
