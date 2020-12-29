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
	"net/http"
	"net/url"
	"os"
	"path"
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
Free:
	for _, v := range m.Versions {
		if v.ID == versionId {
			version = &v
			break Free
		}
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