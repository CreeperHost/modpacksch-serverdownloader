package main

import (
	"crypto"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/cavaliercoder/grab"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var client = &http.Client{}

const BaseAPIURL = "https://api.modpacks.ch/"
const BaseModpackURL = BaseAPIURL + "public/modpack/"
const SearchURL = BaseModpackURL + "search/5?term="
var (
	inProgress = 0
	succeeded = 0
	failed = 0
	parsedArgs = make(map[string]interface{})
)

/*func initWindows() {
	mode := uint32(0)
	if err := windows.GetConsoleMode(windows.Stdout, &mode); err != nil {
		return
	}

	mode = mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	_ = windows.SetConsoleMode(windows.Stdout, mode)
}*/

func main() {
	//initWindows() // todo: more work on making it look nice and work properly, and avoid doing it unless windows 10
	filename := filepath.Base(os.Args[0])
	filename = "serverinstall_61_191"
	var (
		auto = false
		tempPath = ""
		startScript = false
		help = false
	)
	flag.BoolVar(&auto, "auto", auto, "Ask no questions, use defaults.")
	flag.StringVar(&tempPath, "path", tempPath, "Directory to install in. Default: current `directory`")
	flag.BoolVar(&startScript, "startScript", startScript, "Generate a start script using specifications. Default: false")
	flag.BoolVar(&help,"help", help, "This help screen")
	flag.Parse()
	parsedArgs["auto"] = auto
	parsedArgs["path"] = tempPath
	parsedArgs["startScript"] = startScript
	if help {
		flag.Usage()
		os.Exit(0)
	}
	HandleLaunch(filename)
}

/*func Search(term string) []Modpack {
	termSafe := url.QueryEscape(term)
	result := SearchResult{}
	APICall(SearchURL + termSafe, *result)
	return []Modpack{}
}*/

func HandleLaunch(file string) {
	var installPath = parsedArgs["path"].(string)
	if len(installPath) == 0 {
		response := QuestionFree("current directory", "Where would you like to install the server?")
		if response != "current directory" {
			installPath = response
		}
	}
	if len(installPath) == 0 || installPath[0] != "/"[0] {
		installPath = path.Join(".", installPath)
	}
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		if QuestionYN(true,"Path %s does not exist - want to create it?", installPath) {
			if err := os.MkdirAll(installPath, os.FileMode(755)); err != nil {
				log.Fatalf("An error occured whilst creating the folder %s: %v", installPath, err)
			}
		}
	}
	upgrade := false
	if _, err := os.Stat(path.Join(installPath, "version.json")); !os.IsNotExist(err) {
		upgrade = true
	}
	_ = upgrade // temp

	err, modpackid, versionid := ParseFilename(file)

	if err != nil {
		log.Fatalf("Cannot locate modpack via filename %v", err)
	}

	err, modpack := GetModpack(modpackid)
	if err != nil {
		log.Fatalf("Error fetching modpack: %v", err)
	}


	err, versionInfo := modpack.GetVersion(versionid)
	if err != nil {
		log.Fatalf("Error fetching modpack: %v", err)
	}

	downloads := versionInfo.GetDownloads()

	err, ml := versionInfo.GetModLoader()

	if err != nil {
		log.Fatalf("Error getting Modloader: %v", err)
	}

	modLoaderDls := ml.GetDownloads(installPath)

	downloads = append(downloads, modLoaderDls...)

	grabs, err := GetBatch(64, installPath, downloads...)
	if err != nil {
		log.Fatal(err)
	}
	responses := make([]*grab.Response, 0, len(downloads))
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()

	Loop:
	for {
		select {
		case resp := <-grabs:
			if resp != nil {
				// a new response has been received and has started downloading
				responses = append(responses, resp)
			} else {
				// channel is closed - all downloads are complete
				updateUI(responses)
				break Loop
			}

		case <-t.C:
			// update UI every 200ms
			updateUI(responses)
		}
	}

	fmt.Printf(
		"Finished %d successful, %d failed, %d incomplete.\n",
		succeeded,
		failed,
		inProgress)

	ml.Install(installPath)

	// return the number of failed downloads as exit code
	os.Exit(failed)
}

func ParseFilename(file string) (error, int, int) {
	re := regexp.MustCompile("^serverinstall_(\\d+)_(\\d+)")
	matched := re.FindStringSubmatch(file)
	modpackid, err := strconv.Atoi(matched[1])
	if err != nil {
		return errors.New("unable to parse filename: " + file), -1, -1
	}
	versionid, err := strconv.Atoi(matched[2])
	if err != nil {
		return errors.New("unable to parse filename: " + file), -1, -1
	}
	return nil, modpackid, versionid
}

func GetModpack(id int) (error, Modpack) {
	ret := Modpack{}
	newurl := fmt.Sprintf(BaseModpackURL+"%d", id)
	err, newmodpack := APICall(newurl)
	if err != nil {
		return err, ret
	}
	err = json.Unmarshal(newmodpack, &ret)
	if err != nil {
		return err, ret
	}
	if ret.Status == "error" {
		return errors.New(ret.Message), ret
	}
	return nil, ret
}

func (m Modpack) GetVersion(versionid int) (error, VersionInfo) {
	ret := VersionInfo{}
	var version *Version
	Free:
	for _, v := range m.Versions {
		if v.ID == versionid {
			version = &v
			break Free
		}
	}
	if version == nil {
		return errors.New("version does not exist"), ret
	}

	newUrl := fmt.Sprintf(BaseModpackURL+"%d/%d", m.ID, version.ID)
	err, resp := APICall(newUrl)
	if err != nil {
		return err, ret
	}
	err = json.Unmarshal(resp, &ret)
	if err != nil {
		return err, ret
	}

	if ret.Status == "error" {
		return errors.New(ret.Message), ret
	}
	return nil, ret
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
		downloads = append(downloads, Download{f.Path, *parse, v.Name, f.SHA1})
	}
	return downloads
}

func (v VersionInfo) GetModLoader() (error, ModLoader) {
	var ret ModLoader
	var modloader Target
	var minecraftTar Target

	for _, target := range v.Targets {
		if target.Type == "modloader" {
			modloader = target
		}
		if target.Type == "game" {
			minecraftTar = target
		}
	}
	mc := Minecraft{}
	mc.RawVersion = minecraftTar.Version
	if err := mc.Parse(); err != nil {
		log.Fatalf("Error parsing Version: %v", err)
	}

	if modloader.Name == "forge" {
		return GetForge(modloader, mc)
	}
	return errors.New(fmt.Sprintf("Unable to locate modloader for %s %s %s", modloader.Name, modloader.Version, mc.RawVersion)), ret
}

func APICall(url string) (error, []byte) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return err, nil
	}

	stringRet, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err, nil
	}

	resp.Body.Close()

	return nil, stringRet
}

func GetBatch(workers int, dst string, urlStrs ...Download) (<-chan *grab.Response, error) {
	fi, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("destination is not a directory")
	}

	reqs := make([]*grab.Request, len(urlStrs))
	for i := 0; i < len(urlStrs); i++ {
		req, err := grab.NewRequest(path.Join(dst, urlStrs[i].Path, urlStrs[i].Name), urlStrs[i].URL.String())
		if err != nil {
			return nil, err
		}
		if len(urlStrs[i].SHA1) > 0 {
			byteHex, _ := hex.DecodeString(urlStrs[i].SHA1)
			req.SetChecksum(crypto.SHA1.New(), byteHex, false)
		}

		reqs[i] = req
	}

	ch := grab.DefaultClient.DoBatch(workers, reqs...)
	return ch, nil
}


func updateUI(responses []*grab.Response) {
	// clear lines for incomplete downloads
	if inProgress > 0 {
		fmt.Printf("\033[%dA\033[K", inProgress)
	}

	// print newly completed downloads
	for i, resp := range responses {
		if resp != nil && resp.IsComplete() {
			if resp.Err() != nil {
				failed++
				fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n",
					resp.Request.URL(),
					resp.Err())
			} else {
				succeeded++
				fmt.Printf("Finished %s %d / %d bytes (%d%%)\n",
					resp.Filename,
					resp.BytesComplete(),
					resp.Size,
					int(100*resp.Progress()))
			}
			responses[i] = nil
		}
	}

	// print progress for incomplete downloads
	inProgress = 0
	for _, resp := range responses {
		if resp != nil {
			fmt.Printf("Downloading %s %d / %d bytes (%d%%) - %.02fKBp/s ETA: %ds \033[K\n",
				resp.Filename,
				resp.BytesComplete(),
				resp.Size,
				int(100*resp.Progress()),
				resp.BytesPerSecond()/1024,
				int64(resp.ETA().Sub(time.Now()).Seconds()))
			inProgress++
		}
	}
}