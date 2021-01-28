package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
)

type APIFunctions interface {
	GetError() error
}

type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SearchResult struct {
	*APIResponse
	PackIDs []int `json:"packs"`
}

func (resp APIResponse) GetError() error {
	if resp.Status == "error" {
		return errors.New(resp.Message)
	}
	return nil
}

type Modpack struct {
	*APIResponse
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Versions []Version `json:"versions"`
}

type Version struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Updated int    `json:"updated"`
	Specs   Specs  `json:"specs"`
}

type Specs struct {
	Minimum   int `json:"minimum"`
	Recommend int `json:"recommended"`
}

type VersionInfo struct {
	*APIResponse
	*Version
	Files []File  `json:"files"`
	ParentId int  `json:"parent"`
	Targets []Target `json:"targets"`
	RawJson string
}

type Target struct {
	Name string    `json:"name"`
	Version string `json:"Version"`
	Type string    `json:"type"`
}

type File struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Path       string `json:"path"`
	Size       int    `json:"size"`
	ClientOnly bool   `json:"clientonly"`
	ServerOnly bool   `json:"serveronly"`
	Optional   bool   `json:"optional"`
	SHA1       string `json:"sha1"`
}

type Download struct {
	Path string
	URL url.URL
	Name string
	SHA1 string
	FullPath string
}


func GetModpack(id int) (error, Modpack) {
	ret := Modpack{}
	newUrl := fmt.Sprintf(BaseModpackURL+"%d", id)
	err := APICall(newUrl, &ret)
	if err != nil {
		return err, ret
	}
	return ret.GetError(), ret
}

func (m Modpack) GetVersion(versionId int) (error, VersionInfo) {
	var version *Version
	var ret VersionInfo
	latest := false
	if versionId == -2 {
		latest = true
	}
	highestId := -1
	var highestVer *Version
	for _, v := range m.Versions {
		if latest {
			if v.ID > highestId {
				highestId = v.ID
				highestVer = &v
			}
		} else {
			if v.ID == versionId {
				version = &v
				break
			}
		}
	}
	if latest {
		version = highestVer
	}
	if version == nil {
		return errors.New("version does not exist"), ret
	}

	newUrl := fmt.Sprintf(BaseModpackURL+"%d/%d", m.ID, version.ID)
	err := APICall(newUrl, &ret)
	if err != nil {
		return err, ret
	}

	return ret.GetError(), ret
}

func (v VersionInfo) GetDownloads() []Download {
	var downloads []Download
	for _, f := range v.Files {
		if f.ClientOnly {
			continue
		}
		parse, err := url.Parse(f.URL)
		if err != nil {
			//shrug
			continue
		}
		downloads = append(downloads, Download{f.Path, *parse, f.Name, f.SHA1, path.Join(f.Path, f.Name)})
	}
	return downloads
}

func (v VersionInfo) WriteJson(installPath string) bool {
	newUrl := fmt.Sprintf(BaseModpackURL+"%d/%d", v.ParentId, v.ID)
	req, err := http.NewRequest("GET", newUrl, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	stringRet, err := ioutil.ReadAll(resp.Body)

	defer resp.Body.Close()

	return ioutil.WriteFile(path.Join(installPath, "version.json"), stringRet, 644) == nil
}

func (v VersionInfo) WriteStartScript(installPath string, loader ModLoader) {
	jar := loader.GetLaunchJar(installPath)
	launch := fmt.Sprintf("-XX:+UseG1GC -XX:+UnlockExperimentalVMOptions -Xmx%dM -Xms%dM -jar %s nogui", v.Specs.Recommend, v.Specs.Minimum, jar)
	var script string
	filename := "start"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\n" +
			"IF EXIST eula.txt (\r\n" +
			"  goto CHECKEULA\r\n" +
			") ELSE (\r\n" +
			"  goto ASKEULA\r\n" +
			")\r\n" +
			"IF %errlevel% EQU 1 goto END\r\n" +
			":CHECKEULA\r\n" +
			">nul find \"eula=false\" eula.txt && (\r\n" +
			"  goto ASKEULA\r\n" +
			") || (\r\n" +
			"  goto END\r\n" +
			")\r\n" +
			":ASKEULA\r\n" +
			"echo \"Do you agree to the Mojang EULA available at https://account.mojang.com/documents/minecraft_eula ?\"\r\n" +
			"set /p EULA=[y/n]\r\n" +
			"IF /I \"%EULA%\" NEQ \"y\" GOTO END\r\n" +
			"echo eula=true>eula.txt\r\n" +
			":END\r\n" +
			"java.exe " + launch + "\r\n" +
			"pause"
			filename += ".bat"
	} else {
		script = "#!/bin/bash\n" +
		"if ! grep -q \"eula=true\" eula.txt; then\n" +
		"    echo \"Do you agree to the Mojang EULA available at https://account.mojang.com/documents/minecraft_eula ?\"\n" +
		"    read  -n 1 -p \"[y/n] \" EULA\n" +
		"    if [ \"$EULA\" = \"y\" ]; then\n" +
		"        echo \"eula=true\" > eula.txt\n" +
		"        echo\n" +
		"    fi\n" +
		"fi\n" +
		"java " + launch
		filename += ".sh"
	}
	if err := ioutil.WriteFile(path.Join(installPath, filename), []byte(script), 0755); err != nil {
		log.Println(fmt.Sprintf("Error occurred whilst creating launch script: %v", err))
	}
}

func GetVersionInfoFromFile(file string) (error, VersionInfo) {
	ret := VersionInfo{}
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return err, ret
	}
	return json.Unmarshal(bytes, &ret), ret
}

func (d Download) VerifyChecksum(installPath string) bool {
	if len(d.SHA1) == 0 {
		return true
	}

	hasher := sha1.New()

	filename := path.Join(installPath, d.Path, d.Name)

	f, err := os.Open(filename)
	if err != nil {
		return true
	}
	defer f.Close()
	if _, err := io.Copy(hasher, f); err != nil {
		return false
	}

	calcedHash := hasher.Sum(nil)
	decodedBytes, err := hex.DecodeString(d.SHA1)
	return bytes.Compare(calcedHash, decodedBytes) == 0
}