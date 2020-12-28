package main

import (
	"errors"
	"net/url"
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
}