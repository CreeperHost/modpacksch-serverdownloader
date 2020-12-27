package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
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
	if parsedArgs["auto"] == true {
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

	fmt.Println(fmt.Sprintf(s, fmtArgs...) + fmt.Sprintf(" " + choicesFmt, choicesInt...))
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

func (m* Minecraft) Parse() error {
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

	for _, file := range reader.File {
		if file.Name != filePath {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer fileReader.Close()

		bytes, err := ioutil.ReadAll(fileReader)
		if err != nil {
			return nil, err
		}
		return bytes, nil
	}

	return nil, errors.New(fmt.Sprintf("file %s not found in archive %s", filePath, archive))
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
	ID string `json:"id"`
	URL string `json:"url"`
}

type VanillaManifest struct {
	Downloads struct {
		Server struct {
			SHA1 string
			URL string
		} `json:"server"`
	} `json:"downloads"`
}

func (m Minecraft) GetVanillaVersion() (VanillaVersion, error) {
	var ret VanillaVersion
	resp, err := http.Get(MinecraftMetaURL)
	if err == nil {
		defer resp.Body.Close()
		bytes, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			var manifest VanillaListManifest
			err := json.Unmarshal(bytes, &manifest)
			if err == nil {
				Stuff:
				for _, v := range manifest.Versions {
					if v.ID == m.RawVersion {
						ret = v
						break Stuff
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
		bytes, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			vanillaManifest := VanillaManifest{}
			err = json.Unmarshal(bytes, &vanillaManifest)
			if err == nil {
				URL, err := url.Parse(vanillaManifest.Downloads.Server.URL)
				if err == nil {
					ret = Download{"", *URL, "minecraft_server." + v.ID + ".jar", vanillaManifest.Downloads.Server.SHA1}
				}
			}
		}
	}
	return ret, err
}